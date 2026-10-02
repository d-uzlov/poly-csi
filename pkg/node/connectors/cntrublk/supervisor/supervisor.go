package supervisor

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"meoe.io/poly-csi/pkg/logs"
	"meoe.io/poly-csi/pkg/transport"
	"meoe.io/poly-csi/pkg/utils/ublk"
)

type UblkSupervisor struct {
	mut sync.Mutex

	ctx context.Context
	tr  transport.Transport

	configDir      string
	pool           IdPool
	servicesByUuid map[string]*Service
}

type Service struct {
	// initMut is to prevent parallel initialization
	// after initialization the only thread to write values is [ServiceInfo.runLoop]
	initMut sync.Mutex `json:"-"`

	// fields below are filled by [UblkSupervisor.serviceRef]
	tr        transport.Transport `json:"-"`
	ctx       context.Context     `json:"-"`
	cancel    context.CancelFunc  `json:"-"`
	ConfigDir string              `json:"configDir"`

	// Info is populated after device is first created
	Info *ublk.DeviceInfo `json:"info"`

	// fields below should be filled by caller before calling [UblkSupervisor.runLoop]
	Uuid         string            `json:"uuid"`
	DevId        string            `json:"devId"`
	CreateParams ublk.CreateParams `json:"createParams"`
	ExtraParams  ublk.ExtraParams  `json:"-"`
}

const supervisorConfigPrefix = "supervisor-"

func New(ctx context.Context, tr transport.Transport, configDir string, minId int, maxId int) *UblkSupervisor {
	return &UblkSupervisor{
		ctx:            ctx,
		tr:             tr,
		configDir:      configDir,
		pool:           *NewIdPool(minId, maxId),
		servicesByUuid: map[string]*Service{},
	}
}

func (s *UblkSupervisor) StartupRestore() error {
	if s.configDir == "" {
		return fmt.Errorf("config dir is not set")
	}
	ctx := s.ctx
	files, err := s.tr.Cmds().Ls(ctx, s.configDir)
	if err != nil {
		return fmt.Errorf("list config dir: %w", err)
	}
	for _, f := range files {
		if !strings.HasPrefix(f, supervisorConfigPrefix) {
			continue
		}
		data, err := s.tr.Cmds().Read(ctx, s.configDir+"/"+f, false)
		if err != nil {
			return fmt.Errorf("failed to read file: '%s': %w", f, err)
		}
		var svc Service
		err = json.Unmarshal(data, &svc)
		if err != nil {
			return fmt.Errorf("failed to unmarshal json from '%s': %w", f, err)
		}
		svc.tr = s.tr
		svc.ConfigDir = s.configDir
		svc.ctx, svc.cancel = context.WithCancel(s.ctx)
		// TODO check if device even exists before doing recover?
		// TODO check if pid file exists
		go svc.recoverLoop()
	}
	return nil
}

func (s *UblkSupervisor) saveConfig(svc *Service) error {
	data, err := json.Marshal(svc)
	if err != nil {
		return err
	}
	savePath := s.configDir + "/" + supervisorConfigPrefix + svc.Uuid + ".json"
	return s.tr.Cmds().Write(s.ctx, savePath, data)
}

func (s *UblkSupervisor) deleteConfig(devId string, uuid string) error {
	savePath := s.configDir + "/" + supervisorConfigPrefix + uuid + ".json"
	err := s.tr.Cmds().RmFile(s.ctx, savePath)
	if err != nil {
		return err
	}
	return s.tr.Cmds().RmFile(s.ctx, fmt.Sprintf("%s/%s.pid", s.configDir, devId))
}

func (s *UblkSupervisor) Launch(ctx context.Context, uuid string, cp ublk.CreateParams, ep ublk.ExtraParams) (result ublk.DeviceInfo, err error) {
	svc := s.serviceRef(uuid)
	svc.initMut.Lock()
	defer svc.initMut.Unlock()

	if svc.ctx.Err() != nil {
		return ublk.DeviceInfo{}, fmt.Errorf("parent context cancelled or parallel launch error, need to repeat request")
	}

	if svc.Info != nil {
		return *svc.Info, nil
	}

	devIndex, err := s.pool.Allocate()
	if err != nil {
		return ublk.DeviceInfo{}, fmt.Errorf("allocate device id: %w", err)
	}

	logs.FromCtx(ctx).Info("allocated devId", "uuid", uuid, "devId", devIndex)
	svc.DevId = strconv.Itoa(devIndex)
	svc.CreateParams = cp
	svc.CreateParams.DevNum = svc.DevId
	svc.ExtraParams = ep

	defer func() {
		if err == nil {
			err = s.saveConfig(svc)
			if err != nil {
				err = fmt.Errorf("save config on success: %w", err)
			}
		} else {
			s.Remove(uuid)
			delErr := s.deleteConfig(svc.DevId, uuid)
			if delErr != nil {
				logs.FromCtx(ctx).Error("failed to remove config files on error: %w", err)
			}
		}
	}()

	err = s.saveConfig(svc)
	if err != nil {
		return ublk.DeviceInfo{}, fmt.Errorf("save config before starting ublk: %w", err)
	}

	errCh := make(chan error, 1)

	go svc.runLoop(errCh)

	select {
	case <-ctx.Done():
		logs.FromCtx(ctx).Error("context cancelled when waiting for runLoop() result")
		svc.cancel()
	case err := <-errCh:
		if err != nil {
			return ublk.DeviceInfo{}, err
		}
		if svc.Info == nil {
			return ublk.DeviceInfo{}, fmt.Errorf("daemon thread did not set device info")
		}
		return *svc.Info, nil
	}

	select {
	case <-time.Tick(time.Second):
		return ublk.DeviceInfo{}, fmt.Errorf("timed out waiting for daemon process to finish")
	case err := <-errCh:
		if err != nil {
			return ublk.DeviceInfo{}, err
		}
		return ublk.DeviceInfo{}, fmt.Errorf("got nil error from runLoop after cancelling context")
	}
}

func (s *UblkSupervisor) Remove(uuid string) error {
	s.mut.Lock()
	defer s.mut.Unlock()

	svc, ok := s.servicesByUuid[uuid]
	if !ok {
		return nil
	}
	if svc.cancel != nil {
		svc.cancel()
	}
	snip := ublk.New(s.tr)
	err := snip.Delete(s.ctx, svc.DevId)
	if err != nil {
		return fmt.Errorf("delete ublk device: %w", err)
	}
	if svc.DevId != "" && svc.Uuid != "" {
		err := s.deleteConfig(svc.DevId, svc.Uuid)
		if err != nil {
			return err
		}
	}
	delete(s.servicesByUuid, uuid)
	return nil
}

func (s *UblkSupervisor) serviceRef(uuid string) *Service {
	s.mut.Lock()
	defer s.mut.Unlock()

	svc := s.servicesByUuid[uuid]
	if svc != nil {
		return svc
	}
	svc = &Service{
		tr:        s.tr,
		Uuid:      uuid,
		ConfigDir: s.configDir,
	}
	svc.ctx, svc.cancel = context.WithCancel(s.ctx)

	s.servicesByUuid[uuid] = svc
	return svc
}

func (si *Service) runLoop(errCh chan<- error) {
	ctx := si.ctx

	errChClosed := false
	safeClose := func(err error) {
		if errChClosed {
			return
		}
		errCh <- err
		close(errCh)
		errChClosed = true
	}
	defer safeClose(nil)

	snip := ublk.New(si.tr)
	logger := logs.FromCtx(ctx)

	err := snip.CreateForeground(ctx, si.ConfigDir, si.CreateParams, si.ExtraParams, func(info *ublk.DeviceInfo) {
		si.Info = info
		if info != nil {
			safeClose(nil)
			logger.MicroOp("ublk daemon pushed device info", "devId", si.DevId, "info", info)
		}
	}, func(s string) {
		logger.Info("ublk daemon log line", "devId", si.DevId, "line", s)
	})

	if err != nil {
		if !errChClosed {
			safeClose(err)
			return
		}
		// TODO remove this when we intentionally stopped?
		logger.Error("ublk daemon finished with error", "devId", si.DevId, "error", err)
	}

	si.recoverLoop()
}

func (si *Service) recoverLoop() {
	ctx := si.ctx
	snip := ublk.New(si.tr)
	logger := logs.FromCtx(ctx)

	for ctx.Err() == nil {
		logger.MicroOp("restarting ublk daemon", "devId", si.DevId)
		start := time.Now()

		err := snip.RecoverForeground(ctx, si.ConfigDir, si.CreateParams, func(info *ublk.DeviceInfo) {
			if info == nil {
				// RecoverForeground will soon return with an error
				// TODO or will it? there is no one to cancel the context
				return
			}
			logger.MicroOp("recovery: successfully reconnected", "devId", si.Info.DevId, "info", info)
		}, func(s string) {
			logger.Info("ublk daemon log line", "devId", si.DevId, "line", s)
		})

		logger.Error("ublk daemon finished with error", "devId", si.DevId, "error", err)
		timeToSleep := time.Second - time.Since(start)
		if timeToSleep > 0 {
			select {
			case <-ctx.Done():
			case <-time.Tick(timeToSleep):
			}
		}
	}
}

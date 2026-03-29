package nvmetkeeper

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"meoe.io/poly-csi/pkg/transport"
	"meoe.io/poly-csi/pkg/utils/nvmet"
)

type ConfigStorage struct {
	tr           transport.Transport
	configFolder string
	useSudo      bool
	saveState    bool
}

type savedSubsys struct {
	subsys        nvmet.Subsys
	namespaces    []nvmet.Ns
	attachedPorts []string
	attachedHosts []string
}

func NewConfigStorage(tr transport.Transport, configFolder string, useSudo bool, saveState bool) ConfigStorage {
	return ConfigStorage{
		tr:           tr,
		configFolder: configFolder,
		useSudo:      useSudo,
		saveState:    saveState,
	}
}

func (k *ConfigStorage) listPorts(ctx context.Context) ([]nvmet.Port, error) {
	cmds := k.tr.Cmds()

	path := fmt.Sprintf("%s/ports/", k.configFolder)
	portsList, err := cmds.Ls(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("list '%s': %w", path, err)
	}

	var ports []nvmet.Port
	for _, v := range portsList {
		path := fmt.Sprintf("%s/ports/%s", k.configFolder, v)
		stdout, err := cmds.Read(ctx, path, false)
		if err != nil {
			return nil, fmt.Errorf("read '%s': %w", path, err)
		}
		if stdout == nil {
			continue
		}
		var port nvmet.Port
		err = json.Unmarshal(stdout, &port)
		if err != nil {
			return nil, fmt.Errorf("unmarshal '%s' as json: %w", v, err)
		}
		ports = append(ports, port)
	}
	return ports, nil
}

func (k *ConfigStorage) listHosts(ctx context.Context) ([]nvmet.Host, error) {
	cmds := k.tr.Cmds()

	path := fmt.Sprintf("%s/hosts/", k.configFolder)
	hostsList, err := cmds.Ls(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("list '%s': %w", path, err)
	}

	var hosts []nvmet.Host
	for _, v := range hostsList {
		path := fmt.Sprintf("%s/hosts/%s", k.configFolder, v)
		stdout, err := cmds.Read(ctx, path, false)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		if stdout == nil {
			continue
		}
		var host nvmet.Host
		err = json.Unmarshal(stdout, &host)
		if err != nil {
			return nil, fmt.Errorf("unmarshal '%s' as json: %w", v, err)
		}
		// value restoration for cases when older version saved corrupted config
		host.HostKey = strings.TrimSpace(host.HostKey)
		host.ControllerKey = strings.TrimSpace(host.ControllerKey)
		hosts = append(hosts, host)
	}
	return hosts, nil
}

func (k *ConfigStorage) listSubsystems(ctx context.Context) ([]savedSubsys, error) {
	cmds := k.tr.Cmds()

	path := fmt.Sprintf("%s/subsystems/", k.configFolder)
	subsystemsList, err := cmds.Ls(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("list '%s': %w", path, err)
	}

	var subsystems []savedSubsys
	for _, v := range subsystemsList {
		path := fmt.Sprintf("%s/subsystems/%s/subsys.json", k.configFolder, v)
		sr, err := cmds.Stat(ctx, path)
		if err != nil {
			return nil, fmt.Errorf("stat '%s': %w", path, err)
		}
		if !sr.FileExists {
			continue
		}
		stdout, err := cmds.Read(ctx, path, false)
		if err != nil {
			return nil, fmt.Errorf("read '%s': %w", path, err)
		}
		if stdout == nil {
			continue
		}
		var subsys nvmet.Subsys
		err = json.Unmarshal(stdout, &subsys)
		if err != nil {
			return nil, fmt.Errorf("unmarshal subsystem '%s' as json: %w", v, err)
		}

		path = fmt.Sprintf("%s/subsystems/%s/namespaces/", k.configFolder, v)
		namespacesList, err := cmds.Ls(ctx, path)
		if err != nil {
			return nil, fmt.Errorf("list '%s': %w", path, err)
		}

		var namespaces []nvmet.Ns
		for _, n := range namespacesList {
			path := fmt.Sprintf("%s/subsystems/%s/namespaces/%v", k.configFolder, v, n)
			stdout, err = cmds.Read(ctx, path, false)
			if err != nil {
				return nil, fmt.Errorf("read %s: %w", path, err)
			}
			if stdout == nil {
				continue
			}

			var ns nvmet.Ns
			err = json.Unmarshal(stdout, &ns)
			if err != nil {
				return nil, fmt.Errorf("unmarshal namespace '%s/%s' as json: %w", v, n, err)
			}
			namespaces = append(namespaces, ns)
		}

		path = fmt.Sprintf("%s/subsystems/%s/attachments/", k.configFolder, v)
		attachments, err := cmds.Ls(ctx, path)
		if err != nil {
			return nil, fmt.Errorf("list '%s': %w", path, err)
		}

		path = fmt.Sprintf("%s/subsystems/%s/hosts/", k.configFolder, v)
		hosts, err := cmds.Ls(ctx, path)
		if err != nil {
			return nil, fmt.Errorf("list '%s': %w", path, err)
		}

		subsystems = append(subsystems, savedSubsys{
			subsys:        subsys,
			namespaces:    namespaces,
			attachedPorts: attachments,
			attachedHosts: hosts,
		})
	}
	return subsystems, nil
}

func (k *ConfigStorage) savePort(ctx context.Context, c nvmet.Port) error {
	if !k.saveState {
		return nil
	}

	sudoCmds := k.tr.Cmds()

	path := fmt.Sprintf("%s/ports/", k.configFolder)
	err := sudoCmds.Mkdir(ctx, path, true)
	if err != nil {
		return fmt.Errorf("mkdir %s: %w", path, err)
	}

	path = fmt.Sprintf("%s/ports/%s.json", k.configFolder, c.Name)
	encoded, err := json.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal port to json: %w", err)
	}
	err = sudoCmds.Write(ctx, path, encoded)
	if err != nil {
		return fmt.Errorf("write file '%s': %w", path, err)
	}
	return nil
}

func (k *ConfigStorage) deletePortConfig(ctx context.Context, name string) error {
	if !k.saveState {
		return nil
	}

	sudoCmds := k.tr.Cmds()

	path := fmt.Sprintf("%s/ports/%s.json", k.configFolder, name)
	err := sudoCmds.RmFile(ctx, path)
	if err != nil {
		return fmt.Errorf("rm %s: %w", path, err)
	}
	return nil
}

func (k *ConfigStorage) saveNamespace(ctx context.Context, c nvmet.Ns) error {
	if !k.saveState {
		return nil
	}

	sudoCmds := k.tr.Cmds()

	path := fmt.Sprintf("%s/subsystems/%s/namespaces", k.configFolder, c.Nqn)
	err := sudoCmds.Mkdir(ctx, path, false)
	if err != nil {
		return fmt.Errorf("create folder '%s': %w", path, err)
	}

	encoded, err := json.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal port to json: %w", err)
	}
	path = fmt.Sprintf("%s/%s.json", path, c.Nsid)
	err = sudoCmds.Write(ctx, path, encoded)
	if err != nil {
		return fmt.Errorf("write file '%s': %w", path, err)
	}

	return nil
}

func (k *ConfigStorage) deleteNamespace(ctx context.Context, nqn string, nsid string) error {
	if !k.saveState {
		return nil
	}

	sudoCmds := k.tr.Cmds()

	path := fmt.Sprintf("%s/subsystems/%s/namespaces/%s.json", k.configFolder, nqn, nsid)
	err := sudoCmds.RmFile(ctx, path)
	if err != nil {
		return fmt.Errorf("rm %s: %w", path, err)
	}
	return nil
}

func (k *ConfigStorage) saveSubsystem(ctx context.Context, c nvmet.Subsys) error {
	if !k.saveState {
		return nil
	}

	sudoCmds := k.tr.Cmds()

	path := fmt.Sprintf("%s/subsystems/%s", k.configFolder, c.Nqn)
	err := sudoCmds.Mkdir(ctx, path, true)
	if err != nil {
		return fmt.Errorf("mkdir '%s': %w", path, err)
	}

	encoded, err := json.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal port to json: %w", err)
	}
	path = fmt.Sprintf("%s/subsys.json", path)
	err = sudoCmds.Write(ctx, path, encoded)
	if err != nil {
		return fmt.Errorf("write file '%s': %w", path, err)
	}
	return nil
}

func (k *ConfigStorage) deleteSubsystem(ctx context.Context, nqn string) error {
	if !k.saveState {
		return nil
	}

	sudoCmds := k.tr.Cmds()

	path := fmt.Sprintf("%s/subsystems/%s/subsys.json", k.configFolder, nqn)
	err := sudoCmds.RmFile(ctx, path)
	if err != nil {
		return fmt.Errorf("rm %s: %w", path, err)
	}
	return nil
}

func (k *ConfigStorage) saveSubsystemAttachment(ctx context.Context, nqn string, port string) error {
	if !k.saveState {
		return nil
	}

	sudoCmds := k.tr.Cmds()

	path := fmt.Sprintf("%s/subsystems/%s/attachments", k.configFolder, nqn)
	err := sudoCmds.Mkdir(ctx, path, false)
	if err != nil {
		return fmt.Errorf("create folder '%s': %w", path, err)
	}

	path = fmt.Sprintf("%s/%s", path, port)
	err = sudoCmds.Write(ctx, path, nil)
	if err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}

	return nil
}

func (k *ConfigStorage) deleteSubsystemAttachment(ctx context.Context, nqn string, port string) error {
	if !k.saveState {
		return nil
	}

	sudoCmds := k.tr.Cmds()

	path := fmt.Sprintf("%s/subsystems/%s/attachments/%s", k.configFolder, nqn, port)
	err := sudoCmds.RmFile(ctx, path)
	if err != nil {
		return fmt.Errorf("rm %s: %w", path, err)
	}
	return nil
}

func (k *ConfigStorage) saveHost(ctx context.Context, c nvmet.Host) error {
	if !k.saveState {
		return nil
	}

	sudoCmds := k.tr.Cmds()

	path := fmt.Sprintf("%s/hosts/", k.configFolder)
	err := sudoCmds.Mkdir(ctx, path, true)
	if err != nil {
		return fmt.Errorf("mkdir %s: %w", path, err)
	}

	encoded, err := json.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal host to json: %w", err)
	}

	path = fmt.Sprintf("%s/hosts/%s.json", k.configFolder, c.Nqn)
	err = sudoCmds.Write(ctx, path, encoded)
	if err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}

	return nil
}

func (k *ConfigStorage) saveHostAttachment(ctx context.Context, nqn string, host string) error {
	if !k.saveState {
		return nil
	}

	sudoCmds := k.tr.Cmds()

	path := fmt.Sprintf("%s/subsystems/%s/hosts", k.configFolder, nqn)
	err := sudoCmds.Mkdir(ctx, path, false)
	if err != nil {
		return fmt.Errorf("create folder '%s': %w", path, err)
	}

	path = fmt.Sprintf("%s/subsystems/%s/hosts/%s", k.configFolder, nqn, host)
	err = sudoCmds.Write(ctx, path, nil)
	if err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}

	return nil
}

func (k *ConfigStorage) deleteHostAttachment(ctx context.Context, nqn string, host string) error {
	if !k.saveState {
		return nil
	}

	sudoCmds := k.tr.Cmds()

	path := fmt.Sprintf("%s/subsystems/%s/hosts/%s", k.configFolder, nqn, host)
	err := sudoCmds.RmFile(ctx, path)
	if err != nil {
		return fmt.Errorf("rm %s: %w", path, err)
	}
	return nil
}

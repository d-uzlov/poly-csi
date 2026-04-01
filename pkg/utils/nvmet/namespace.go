package nvmet

import (
	"encoding/json"
	"fmt"
)

type Ns struct {
	Nqn        string `json:"nqn"`
	Nsid       string `json:"nsid"`
	DevicePath string `json:"device_path"`
	DeviceUuid string `json:"device_uuid"`
	DeviceGuid string `json:"device_guid"`
	Enabled    bool   `json:"enabled"`
}

func (c Ns) Json() string {
	encoded, err := json.Marshal(c)
	if err != nil {
		panic(err)
	}
	return string(encoded)
}

func (c Ns) Validate() error {
	if c.Nqn == "" {
		return fmt.Errorf("nqn is empty")
	}
	if c.Nsid == "" {
		return fmt.Errorf("nsid is empty")
	}
	if c.DevicePath == "" {
		return fmt.Errorf("device_path is empty")
	}
	return nil
}

func CheckNamespaceEquivalent(requested Ns, existing Ns) bool {
	if requested.Nqn != existing.Nqn {
		return false
	}
	if requested.Nsid != existing.Nsid {
		return false
	}
	if requested.DevicePath != existing.DevicePath {
		return false
	}
	if requested.DeviceUuid != "" && requested.DeviceUuid != existing.DeviceUuid {
		return false
	}
	if requested.DeviceGuid != "" && requested.DeviceGuid != existing.DeviceGuid {
		return false
	}
	return true
}

func (s Snippets) ListNamespaces(subsysNqn string) ([]string, error) {
	path := fmt.Sprintf("/sys/kernel/config/nvmet/subsystems/%s/namespaces", subsysNqn)
	return s.tr.Cmds().Ls(s.ctx, path)
}

func (s Snippets) ReadNamespace(subsysNqn string, nsid string) (*Ns, error) {
	path := fmt.Sprintf("/sys/kernel/config/nvmet/subsystems/%s/namespaces/%s", subsysNqn, nsid)
	stat, err := s.tr.Cmds().Stat(s.ctx, path)
	if err != nil {
		return nil, fmt.Errorf("check if folder '%s' exists: %w", path, err)
	}
	if !stat.ParentExists {
		return nil, fmt.Errorf("folder /sys/kernel/config/nvmet/subsystems/%s/namespaces does not exist", subsysNqn)
	}
	if !stat.FileExists {
		// ns does not exist in nvmet
		return nil, nil
	}

	attrPath := fmt.Sprintf("%s/device_path", path)
	devicePath, err := s.tr.Cmds().Read(s.ctx, attrPath, true)
	if err != nil {
		return nil, fmt.Errorf("read '%s': %w", attrPath, err)
	}

	attrPath = fmt.Sprintf("%s/device_uuid", path)
	deviceUuid, err := s.tr.Cmds().Read(s.ctx, attrPath, true)
	if err != nil {
		return nil, fmt.Errorf("read '%s': %w", attrPath, err)
	}

	attrPath = fmt.Sprintf("%s/device_nguid", path)
	deviceGuid, err := s.tr.Cmds().Read(s.ctx, attrPath, true)
	if err != nil {
		return nil, fmt.Errorf("read '%s': %w", attrPath, err)
	}

	attrPath = fmt.Sprintf("%s/enable", path)
	enabledStdout, err := s.tr.Cmds().Read(s.ctx, attrPath, true)
	if err != nil {
		return nil, fmt.Errorf("read '%s': %w", attrPath, err)
	}

	enabled := false
	switch string(enabledStdout) {
	case "0":
		enabled = false
	case "1":
		enabled = true
	default:
		return nil, fmt.Errorf("unknown state for 'enabled' property: '%s'", string(enabledStdout))
	}

	return &Ns{
		Nqn:        subsysNqn,
		Nsid:       nsid,
		DevicePath: string(devicePath),
		DeviceUuid: string(deviceUuid),
		DeviceGuid: string(deviceGuid),
		Enabled:    enabled,
	}, nil
}

// SetNamespaceEnabled takes full namespace description, but ignores parameters, except for Enabled
func (s Snippets) SetNamespaceEnabled(c Ns) (err error) {
	path := fmt.Sprintf("/sys/kernel/config/nvmet/subsystems/%s/namespaces/%s", c.Nqn, c.Nsid)

	enable := "0"
	if c.Enabled {
		enable = "1"
	}

	attrPath := fmt.Sprintf("%s/enable", path)
	err = s.tr.SudoCmds().Write(s.ctx, attrPath, []byte(enable))
	if err != nil {
		err = fmt.Errorf("write %s to '%s/enable': %w", enable, path, err)
	}

	return nil
}

func (s Snippets) ConfigureNamespace(c Ns) (err error) {
	err = c.Validate()
	if err != nil {
		return fmt.Errorf("validate: %w", err)
	}
	existingNs, err := s.ReadNamespace(c.Nqn, c.Nsid)
	if err != nil {
		return fmt.Errorf("read nvme ns: %w", err)
	}

	path := fmt.Sprintf("/sys/kernel/config/nvmet/subsystems/%s/namespaces/%s", c.Nqn, c.Nsid)

	if existingNs == nil {
		err := s.tr.SudoCmds().Mkdir(s.ctx, path, false)
		if err != nil {
			return fmt.Errorf("create ns '%s': %w", path, err)
		}

		existingNs, err = s.ReadNamespace(c.Nqn, c.Nsid)
		if err != nil {
			return fmt.Errorf("read nvme ns after creation: %w", err)
		}
		if existingNs == nil {
			return fmt.Errorf("nvme ns does not exist despite that we have just created it: %s", path)
		}
	}

	defer func() {
		if err != nil {
			return
		}
		if existingNs.Enabled == c.Enabled {
			return
		}
		enable := "0"
		if c.Enabled {
			enable = "1"
		}
		// we are in defer, we can just set err to fail the function
		err = s.WriteAttribute(path, "enable", enable)
	}()

	// if all parameters already match, don't need to do anything
	if existingNs.DevicePath == c.DevicePath &&
		(c.DeviceGuid == "" || existingNs.DeviceGuid == c.DeviceGuid) &&
		(c.DeviceUuid == "" || existingNs.DeviceUuid == c.DeviceUuid) {
		return nil
	}

	// need to disable NS before configuring it
	if existingNs.Enabled {
		err = s.WriteAttribute(path, "enable", "0")
		if err != nil {
			return fmt.Errorf("disable ns before changes: %w", err)
		}
		existingNs.Enabled = false
	}

	err = s.WriteAttributes(path, map[string]string{
		"device_path":  c.DevicePath,
		"device_uuid":  c.DeviceUuid,
		"device_nguid": c.DeviceGuid,
	})
	if err != nil {
		return fmt.Errorf("set attributes: %w", err)
	}
	return nil
}

func (s Snippets) DeleteNamespace(subsysNqn string, nsid string) error {
	path := fmt.Sprintf("/sys/kernel/config/nvmet/subsystems/%s/namespaces/%s/", subsysNqn, nsid)
	err := s.tr.SudoCmds().RmDir(s.ctx, path)
	if err != nil {
		return fmt.Errorf("remove %s: %w", path, err)
	}
	return nil
}

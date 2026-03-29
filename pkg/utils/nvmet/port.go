package nvmet

import (
	"fmt"
	"strconv"
)

type Port struct {
	// name must be a valid integer, but it's stored as string for convenience
	Name          string `json:"name"`
	AddressFamily string `json:"address_family"`
	ListenAddr    string `json:"listen_addr"`
	ListenPort    int    `json:"listen_port"`
	TransportType string `json:"transport_type"`
}

func (c Port) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("name is empty")
	}
	if c.AddressFamily == "" {
		return fmt.Errorf("address_family is empty")
	}
	if c.ListenAddr == "" {
		return fmt.Errorf("listen_addr is empty")
	}
	if c.ListenPort == 0 {
		return fmt.Errorf("listen_port is empty")
	}
	if c.TransportType == "" {
		return fmt.Errorf("transport_type is empty")
	}
	return nil
}

func (s Snippets) ListPorts() ([]string, error) {
	path := "/sys/kernel/config/nvmet/ports/"
	return s.tr.Cmds().Ls(s.ctx, path)
}

func (s Snippets) ReadPort(name string) (*Port, error) {
	path := fmt.Sprintf("/sys/kernel/config/nvmet/ports/%s", name)
	stat, err := s.tr.Cmds().Stat(s.ctx, path)
	if err != nil {
		return nil, fmt.Errorf("stat '%s': %w", path, err)
	}
	if !stat.ParentExists {
		return nil, fmt.Errorf("folder /sys/kernel/config/nvmet/ports does not exist")
	}
	if !stat.FileExists {
		// port does not exist in nvmet
		return nil, nil
	}

	attrPath := fmt.Sprintf("%s/addr_adrfam", path)
	addrFam, err := s.tr.Cmds().Read(s.ctx, attrPath, true)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", attrPath, err)
	}
	attrPath = fmt.Sprintf("%s/addr_traddr", path)
	listenAddr, err := s.tr.Cmds().Read(s.ctx, attrPath, true)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", attrPath, err)
	}
	attrPath = fmt.Sprintf("%s/addr_trsvcid", path)
	listenPortRaw, err := s.tr.Cmds().Read(s.ctx, attrPath, true)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", attrPath, err)
	}
	attrPath = fmt.Sprintf("%s/addr_trtype", path)
	transportType, err := s.tr.Cmds().Read(s.ctx, attrPath, true)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", attrPath, err)
	}

	listenPort, err := strconv.Atoi(string(listenPortRaw))
	if err != nil {
		return nil, fmt.Errorf("parse port '%s' as int: %w", listenPortRaw, err)
	}

	return &Port{
		Name:          name,
		AddressFamily: string(addrFam),
		ListenAddr:    string(listenAddr),
		ListenPort:    listenPort,
		TransportType: string(transportType),
	}, nil
}

func (s Snippets) ConfigurePort(c Port) error {
	err := c.Validate()
	if err != nil {
		return fmt.Errorf("validate: %w", err)
	}
	path := fmt.Sprintf("/sys/kernel/config/nvmet/ports/%s", c.Name)
	err = s.tr.SudoCmds().Mkdir(s.ctx, path, false)
	if err != nil {
		return fmt.Errorf("mkdir %s: %w", path, err)
	}

	attrPath := fmt.Sprintf("%s/addr_adrfam", path)
	err = s.tr.SudoCmds().Write(s.ctx, attrPath, []byte(c.AddressFamily))
	if err != nil {
		return fmt.Errorf("write '%s' to %s/addr_adrfam: %w", c.AddressFamily, path, err)
	}
	attrPath = fmt.Sprintf("%s/addr_traddr", path)
	err = s.tr.SudoCmds().Write(s.ctx, attrPath, []byte(c.ListenAddr))
	if err != nil {
		return fmt.Errorf("write '%s' to %s/addr_traddr (listening address): %w", c.ListenAddr, path, err)
	}
	attrPath = fmt.Sprintf("%s/addr_trsvcid", path)
	err = s.tr.SudoCmds().Write(s.ctx, attrPath, []byte(strconv.FormatInt(int64(c.ListenPort), 10)))
	if err != nil {
		return fmt.Errorf("write %d to %s/addr_trsvcid (listening port): %w", c.ListenPort, path, err)
	}
	attrPath = fmt.Sprintf("%s/addr_trtype", path)
	err = s.tr.SudoCmds().Write(s.ctx, attrPath, []byte(c.TransportType))
	if err != nil {
		return fmt.Errorf("write '%s' to %s/addr_trtype: %w", c.TransportType, path, err)
	}
	return nil
}

func (s Snippets) DeletePort(name string) error {
	path := fmt.Sprintf("/sys/kernel/config/nvmet/ports/%s", name)
	return s.tr.SudoCmds().RmDir(s.ctx, path)
}

func (s Snippets) ListPortAttachments(port string) ([]string, error) {
	path := fmt.Sprintf("/sys/kernel/config/nvmet/ports/%s/subsystems/", port)
	return s.tr.Cmds().Ls(s.ctx, path)
}

func (s Snippets) AttachSubsysToPort(nqn string, nvmePort string) error {
	path := fmt.Sprintf("/sys/kernel/config/nvmet/subsystems/%s", nqn)
	link := fmt.Sprintf("/sys/kernel/config/nvmet/ports/%s/subsystems/%s", nvmePort, nqn)
	stat, err := s.tr.Cmds().Stat(s.ctx, link)
	if err != nil {
		return fmt.Errorf("stat '%s' (checking if nqn is already configured on port): %w", link, err)
	}
	if stat.FileExists {
		if !stat.IsDirectory() {
			return fmt.Errorf("link '%s' exists but does not point to a directory", link)
		}
		if stat.Path != path {
			return fmt.Errorf("name clash for '%s': we want to attach '%s' but existing link points to '%s'", link, path, stat.Path)
		}
		// link already points to required path
		return nil
	}
	// link does not exist, need to create it
	err = s.tr.SudoCmds().CreateSymlink(s.ctx, link, path)
	if err != nil {
		return fmt.Errorf("create link '%s' pointing to '%s': %w", path, link, err)
	}
	return nil
}

func (s Snippets) DetachNqnFromPort(nqn string, nvmePort string) error {
	portsLink := fmt.Sprintf("/sys/kernel/config/nvmet/ports/%s/subsystems/%s", nvmePort, nqn)
	return s.tr.SudoCmds().RmFile(s.ctx, portsLink)
}

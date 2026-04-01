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

func (s Snippets) ReadPort(portName string) (*Port, error) {
	path := fmt.Sprintf("/sys/kernel/config/nvmet/ports/%s", portName)
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
		Name:          portName,
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
	err = s.WriteAttributes(path, map[string]string{
		"addr_adrfam":  c.AddressFamily,
		"addr_traddr":  c.ListenAddr,
		"addr_trsvcid": strconv.FormatInt(int64(c.ListenPort), 10),
		"addr_trtype":  c.TransportType,
	})
	if err != nil {
		return fmt.Errorf("set attributes: %w", err)
	}

	return nil
}

func (s Snippets) DeletePort(portName string) error {
	path := fmt.Sprintf("/sys/kernel/config/nvmet/ports/%s", portName)
	return s.tr.SudoCmds().RmDir(s.ctx, path)
}

func (s Snippets) ListPortAttachments(portName string) ([]string, error) {
	path := fmt.Sprintf("/sys/kernel/config/nvmet/ports/%s/subsystems/", portName)
	return s.tr.Cmds().Ls(s.ctx, path)
}

func (s Snippets) AttachSubsysToPort(subsysNqn string, portName string) error {
	path := fmt.Sprintf("/sys/kernel/config/nvmet/subsystems/%s", subsysNqn)
	link := fmt.Sprintf("/sys/kernel/config/nvmet/ports/%s/subsystems/%s", portName, subsysNqn)
	err := s.tr.SudoCmds().EnsureSymlink(s.ctx, link, path)
	if err != nil {
		return err
	}
	return nil
}

func (s Snippets) DetachNqnFromPort(subsysNqn string, portName string) error {
	portsLink := fmt.Sprintf("/sys/kernel/config/nvmet/ports/%s/subsystems/%s", portName, subsysNqn)
	return s.tr.SudoCmds().RmFile(s.ctx, portsLink)
}

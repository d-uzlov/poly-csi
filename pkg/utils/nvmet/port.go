package nvmet

import (
	"fmt"
	"strconv"

	"meoe.io/poly-csi/pkg/transport"
)

type Port struct {
	// name must be a valid integer, but it's stored as string for convenience
	Name          string `json:"name"`
	AddressFamily string `json:"address_family"`
	ListenAddr    string `json:"listen_addr"`
	ListenPort    int    `json:"listen_port"`
	Type          string `json:"type"`
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
	if c.Type == "" {
		return fmt.Errorf("type is empty")
	}
	return nil
}

func (s Snippets) ListPorts() ([]string, error) {
	tr := transport.FromContext(s.ctx)
	path := "/sys/kernel/config/nvmet/ports/"
	return tr.Cmds().Ls(s.ctx, path)
}

func (s Snippets) ReadPort(name string) (*Port, error) {
	tr := transport.FromContext(s.ctx)

	path := fmt.Sprintf("/sys/kernel/config/nvmet/ports/%s", name)
	stat, err := tr.Cmds().Stat(s.ctx, path)
	if err != nil {
		return nil, fmt.Errorf("stat '%s': %w", path, err)
	}
	if stat == nil {
		return nil, nil
	}

	attrPath := fmt.Sprintf("%s/addr_adrfam", path)
	addrFam, err := tr.Cmds().Read(s.ctx, attrPath, true)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", attrPath, err)
	}
	attrPath = fmt.Sprintf("%s/addr_traddr", path)
	listenAddr, err := tr.Cmds().Read(s.ctx, attrPath, true)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", attrPath, err)
	}
	attrPath = fmt.Sprintf("%s/addr_trsvcid", path)
	listenPortRaw, err := tr.Cmds().Read(s.ctx, attrPath, true)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", attrPath, err)
	}
	attrPath = fmt.Sprintf("%s/addr_trtype", path)
	transportType, err := tr.Cmds().Read(s.ctx, attrPath, true)
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
		Type:          string(transportType),
	}, nil
}

func (s Snippets) ConfigurePort(c Port) error {
	tr := transport.FromContext(s.ctx)

	path := fmt.Sprintf("/sys/kernel/config/nvmet/ports/%s", c.Name)
	err := tr.SudoCmds().Mkdir(s.ctx, path, false)
	if err != nil {
		return fmt.Errorf("mkdir %s: %w", path, err)
	}

	attrPath := fmt.Sprintf("%s/addr_adrfam", path)
	err = tr.SudoCmds().Write(s.ctx, attrPath, []byte(c.AddressFamily))
	if err != nil {
		return fmt.Errorf("write '%s' to %s/addr_adrfam: %w", c.AddressFamily, path, err)
	}
	attrPath = fmt.Sprintf("%s/addr_traddr", path)
	err = tr.SudoCmds().Write(s.ctx, attrPath, []byte(c.ListenAddr))
	if err != nil {
		return fmt.Errorf("write '%s' to %s/addr_traddr (listening address): %w", c.ListenAddr, path, err)
	}
	attrPath = fmt.Sprintf("%s/addr_trsvcid", path)
	err = tr.SudoCmds().Write(s.ctx, attrPath, []byte(strconv.FormatInt(int64(c.ListenPort), 10)))
	if err != nil {
		return fmt.Errorf("write %d to %s/addr_trsvcid (listening port): %w", c.ListenPort, path, err)
	}
	attrPath = fmt.Sprintf("%s/addr_trtype", path)
	err = tr.SudoCmds().Write(s.ctx, attrPath, []byte(c.Type))
	if err != nil {
		return fmt.Errorf("write '%s' to %s/addr_trtype: %w", c.Type, path, err)
	}
	return nil
}

func (s Snippets) DeletePort(name string) error {
	tr := transport.FromContext(s.ctx)
	path := fmt.Sprintf("/sys/kernel/config/nvmet/ports/%s", name)
	return tr.SudoCmds().RmDir(s.ctx, path)
}

func (s Snippets) ListPortAttachments(port string) ([]string, error) {
	tr := transport.FromContext(s.ctx)
	path := fmt.Sprintf("/sys/kernel/config/nvmet/ports/%s/subsystems/", port)
	return tr.Cmds().Ls(s.ctx, path)
}

func (s Snippets) AttachNqnToPort(nqn string, nvmePort string) error {
	tr := transport.FromContext(s.ctx)

	path := fmt.Sprintf("/sys/kernel/config/nvmet/subsystems/%s", nqn)
	link := fmt.Sprintf("/sys/kernel/config/nvmet/ports/%s/subsystems/%s", nvmePort, nqn)
	stat, err := tr.Cmds().Stat(s.ctx, link)
	if err != nil {
		return fmt.Errorf("stat '%s' (checking if nqn is already configured on port): %w", link, err)
	}
	if stat != nil {
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
	err = tr.SudoCmds().CreateSymlink(s.ctx, link, path)
	if err != nil {
		return fmt.Errorf("create link '%s' pointing to '%s': %w", path, link, err)
	}
	return nil
}

func (s Snippets) DetachNqnFromPort(nqn string, nvmePort string) error {
	tr := transport.FromContext(s.ctx)
	portsLink := fmt.Sprintf("/sys/kernel/config/nvmet/ports/%s/subsystems/%s", nvmePort, nqn)
	return tr.SudoCmds().RmFile(s.ctx, portsLink)
}

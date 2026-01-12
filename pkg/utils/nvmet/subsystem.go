package nvmet

import (
	"encoding/json"
	"fmt"

	"meoe.io/poly-csi/pkg/transport"
)

type Subsys struct {
	Nqn          string `json:"nqn"`
	Model        string `json:"model"`
	Serial       string `json:"serial"`
	AllowAnyHost bool   `json:"allow_any_host"`
}

func (c Subsys) Validate() error {
	if c.Nqn == "" {
		return fmt.Errorf("nqn is empty")
	}
	return nil
}

func (c Subsys) Json() string {
	encoded, err := json.Marshal(c)
	if err != nil {
		panic(err)
	}
	return string(encoded)
}

func (s Snippets) ListSubsys() ([]string, error) {
	tr := transport.FromContext(s.ctx)
	path := "/sys/kernel/config/nvmet/subsystems/"
	return tr.Cmds().Ls(s.ctx, path)
}

func (s Snippets) ReadSubsys(nqn string) (*Subsys, error) {
	tr := transport.FromContext(s.ctx)

	path := fmt.Sprintf("/sys/kernel/config/nvmet/subsystems/%s", nqn)
	stat, err := tr.Cmds().Stat(s.ctx, path)
	if err != nil {
		return nil, fmt.Errorf("check if folder '%s' exists: %w", path, err)
	}
	if stat == nil {
		return nil, nil
	}

	attrPath := fmt.Sprintf("%s/attr_model", path)
	model, err := tr.Cmds().Read(s.ctx, attrPath, true)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", attrPath, err)
	}
	attrPath = fmt.Sprintf("%s/attr_serial", path)
	serial, err := tr.Cmds().Read(s.ctx, attrPath, true)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", attrPath, err)
	}
	attrPath = fmt.Sprintf("%s/attr_allow_any_host", path)
	allowAnyHostRaw, err := tr.Cmds().Read(s.ctx, attrPath, true)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", attrPath, err)
	}

	allowAnyHost := false
	if string(allowAnyHostRaw) == "1" {
		allowAnyHost = true
	}

	return &Subsys{
		Nqn:          nqn,
		Model:        string(model),
		Serial:       string(serial),
		AllowAnyHost: allowAnyHost,
	}, nil
}

func (s Snippets) ConfigureSubsys(c Subsys) error {
	tr := transport.FromContext(s.ctx)

	path := fmt.Sprintf("/sys/kernel/config/nvmet/subsystems/%s", c.Nqn)
	err := tr.SudoCmds().Mkdir(s.ctx, path, false)
	if err != nil {
		return fmt.Errorf("mkdir %s: %w", path, err)
	}
	allowAnyHost := "0"
	if c.AllowAnyHost {
		allowAnyHost = "1"
	}

	attrPath := fmt.Sprintf("%s/attr_allow_any_host", path)
	err = tr.SudoCmds().Write(s.ctx, attrPath, []byte(allowAnyHost))
	if err != nil {
		return fmt.Errorf("write '%s' to %s/attr_allow_any_host: %w", allowAnyHost, path, err)
	}
	if c.Model != "" {
		attrPath = fmt.Sprintf("%s/attr_model", path)
		err = tr.SudoCmds().Write(s.ctx, attrPath, []byte(c.Model))
		if err != nil {
			return fmt.Errorf("write '%s' to %s/attr_model: %w", c.Model, path, err)
		}
	}
	if c.Serial != "" {
		attrPath = fmt.Sprintf("%s/attr_serial", path)
		err = tr.SudoCmds().Write(s.ctx, attrPath, []byte(c.Serial))
		if err != nil {
			return fmt.Errorf("write '%s' to %s/attr_serial: %w", c.Serial, path, err)
		}
	}
	return nil
}

func (s Snippets) DeleteSubsys(nqn string) error {
	tr := transport.FromContext(s.ctx)
	path := fmt.Sprintf("/sys/kernel/config/nvmet/subsystems/%s/", nqn)
	err := tr.SudoCmds().RmDir(s.ctx, path)
	if err != nil {
		return fmt.Errorf("rmdir %s: %w", path, err)
	}
	return nil
}

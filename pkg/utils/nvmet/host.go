package nvmet

import (
	"encoding/json"
	"fmt"
	"strings"
)

type Host struct {
	Nqn           string `json:"nqn"`
	NeedAuth      bool   `json:"need_auth"`
	HostKey       string `json:"host_key"`
	ControllerKey string `json:"controller_key"`
	HashType      string `json:"hash_type"`
	DhGroup       string `json:"dh_group"`
}

const (
	HashTypeHmacSha256 = "hmac(sha256)"
	HashTypeHmacSha384 = "hmac(sha384)"
	HashTypeHmacSha512 = "hmac(sha512)"

	DhGroupEmpty = "0"
	DhGroup2048  = "ffdhe2048"
	DhGroup3072  = "ffdhe3072"
	DhGroup4096  = "ffdhe4096"
	DhGroup6144  = "ffdhe6144"
	DhGroup8192  = "ffdhe8192"
)

func (c Host) Json() string {
	encoded, err := json.Marshal(c)
	if err != nil {
		panic(err)
	}
	return string(encoded)
}

func (c Host) IsEqual(other Host) bool {
	if c.Nqn != other.Nqn {
		return false
	}
	if c.NeedAuth != other.NeedAuth {
		return false
	}
	if !c.NeedAuth && !other.NeedAuth {
		return true
	}
	// rest of the fields can be compared literally
	return c == other
}

func (c Host) Validate() error {
	if c.Nqn == "" {
		return fmt.Errorf("nqn is empty")
	}
	if c.Nqn != strings.TrimSpace(c.Nqn) {
		return fmt.Errorf("nqn has spaces")
	}
	if c.NeedAuth {
		if c.HostKey == "" {
			return fmt.Errorf("host_key is empty")
		}
		if c.HostKey != strings.TrimSpace(c.HostKey) {
			return fmt.Errorf("host_key has spaces")
		}
		if c.ControllerKey == "" {
			return fmt.Errorf("controller_key is empty")
		}
		if c.ControllerKey != strings.TrimSpace(c.ControllerKey) {
			return fmt.Errorf("controller_key has spaces")
		}
		if c.HashType == "" {
			return fmt.Errorf("hash_type is empty")
		}
		if c.DhGroup == "" {
			return fmt.Errorf("dh_group is empty")
		}
	}
	return nil
}

func (s Snippets) ListHosts() ([]string, error) {
	path := "/sys/kernel/config/nvmet/hosts"
	return s.tr.Cmds().Ls(s.ctx, path)
}

func (s Snippets) ListHostAttachments(nqn string) ([]string, error) {
	path := fmt.Sprintf("/sys/kernel/config/nvmet/subsystems/%s/allowed_hosts", nqn)
	return s.tr.Cmds().Ls(s.ctx, path)
}

func (s Snippets) ReadHost(name string) (*Host, error) {
	path := fmt.Sprintf("/sys/kernel/config/nvmet/hosts/%s", name)
	stat, err := s.tr.Cmds().Stat(s.ctx, path)
	if err != nil {
		return nil, fmt.Errorf("stat '%s': %w", path, err)
	}
	if !stat.ParentExists {
		return nil, fmt.Errorf("folder /sys/kernel/config/nvmet/hosts does not exist")
	}
	if !stat.FileExists {
		// host does not exist in nvmet
		return nil, nil
	}

	// debian 12 linux 6.1 doesn't seem to have DH-CHAP enabled by default
	// debian 13 linux 6.12 and debian 13 linux 6.16 both have this on by default
	attrPath := fmt.Sprintf("%s/dhchap_key", path)
	stat, err = s.tr.Cmds().Stat(s.ctx, attrPath)
	if err != nil {
		return nil, fmt.Errorf("stat '%s': %w", attrPath, err)
	}
	if !stat.FileExists {
		// this server does not support auth
		return &Host{
			Nqn:      name,
			NeedAuth: false,
		}, nil
	}

	attrPath = fmt.Sprintf("%s/dhchap_key", path)
	dhChapKey, err := s.tr.Cmds().Read(s.ctx, attrPath, true)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", attrPath, err)
	}
	attrPath = fmt.Sprintf("%s/dhchap_ctrl_key", path)
	dhChapControllerKey, err := s.tr.Cmds().Read(s.ctx, attrPath, true)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", attrPath, err)
	}
	attrPath = fmt.Sprintf("%s/dhchap_hash", path)
	hashType, err := s.tr.Cmds().Read(s.ctx, attrPath, true)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", attrPath, err)
	}
	attrPath = fmt.Sprintf("%s/dhchap_dhgroup", path)
	dhGroup, err := s.tr.Cmds().Read(s.ctx, attrPath, true)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", attrPath, err)
	}
	dhGroupS := string(dhGroup)
	if dhGroupS == "null" {
		dhGroupS = ""
	}

	needAuth := false
	if string(dhChapKey) != "" {
		needAuth = true
	}

	return &Host{
		Nqn:           name,
		NeedAuth:      needAuth,
		HostKey:       string(dhChapKey),
		ControllerKey: string(dhChapControllerKey),
		HashType:      string(hashType),
		DhGroup:       dhGroupS,
	}, nil
}

func (s Snippets) ConfigureHost(c Host) error {
	err := c.Validate()
	if err != nil {
		return fmt.Errorf("validate: %w", err)
	}
	path := fmt.Sprintf("/sys/kernel/config/nvmet/hosts/%s", c.Nqn)
	err = s.tr.SudoCmds().Mkdir(s.ctx, path, false)
	if err != nil {
		return fmt.Errorf("mkdir %s: %w", path, err)
	}
	if !c.NeedAuth {
		return nil
	}

	// debian 12 linux 6.1 doesn't seem to have DH-CHAP enabled by default
	// debian 13 linux 6.12 and debian 13 linux 6.16 both have this on by default
	attrPath := fmt.Sprintf("%s/dhchap_key", path)
	stat, err := s.tr.Cmds().Stat(s.ctx, attrPath)
	if err != nil {
		return fmt.Errorf("stat '%s': %w", attrPath, err)
	}
	if !stat.FileExists {
		return fmt.Errorf("kernel does not support DH-CHAP: missing file %s", attrPath)
	}

	attrPath = fmt.Sprintf("%s/dhchap_key", path)
	err = s.tr.SudoCmds().Write(s.ctx, attrPath, []byte(c.HostKey))
	if err != nil {
		return fmt.Errorf("write %s: %w", attrPath, err)
	}
	attrPath = fmt.Sprintf("%s/dhchap_ctrl_key", path)
	err = s.tr.SudoCmds().Write(s.ctx, attrPath, []byte(c.ControllerKey))
	if err != nil {
		return fmt.Errorf("write %s: %w", attrPath, err)
	}
	attrPath = fmt.Sprintf("%s/dhchap_hash", path)
	err = s.tr.SudoCmds().Write(s.ctx, attrPath, []byte(c.HashType))
	if err != nil {
		return fmt.Errorf("write '%s' to %s: %w", c.HashType, attrPath, err)
	}
	attrPath = fmt.Sprintf("%s/dhchap_dhgroup", path)
	err = s.tr.SudoCmds().Write(s.ctx, attrPath, []byte(c.DhGroup))
	if err != nil {
		return fmt.Errorf("write '%s' to %s: %w", c.DhGroup, attrPath, err)
	}
	return nil
}

func (s Snippets) AllowHost(subsysNqn string, hostNqn string) error {
	path := fmt.Sprintf("/sys/kernel/config/nvmet/hosts/%s", hostNqn)
	link := fmt.Sprintf("/sys/kernel/config/nvmet/subsystems/%s/allowed_hosts/%s", subsysNqn, hostNqn)
	stat, err := s.tr.Cmds().Stat(s.ctx, link)
	if err != nil {
		return fmt.Errorf("stat '%s' (checking if host is already configured on nqn): %w", link, err)
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

func (s Snippets) DisallowHost(subsysNqn string, hostNqn string) error {
	subsysHostLink := fmt.Sprintf("/sys/kernel/config/nvmet/subsystems/%s/allowed_hosts/%s", subsysNqn, hostNqn)
	err := s.tr.SudoCmds().RmFile(s.ctx, subsysHostLink)
	if err != nil {
		return fmt.Errorf("rm '%s': %w", subsysHostLink, err)
	}
	return nil
}

func (s Snippets) DeleteHost(name string) error {
	path := fmt.Sprintf("/sys/kernel/config/nvmet/hosts/%s", name)
	err := s.tr.SudoCmds().RmDir(s.ctx, path)
	if err != nil {
		return fmt.Errorf("rmdir '%s': %w", path, err)
	}
	return nil
}

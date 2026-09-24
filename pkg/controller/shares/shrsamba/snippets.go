package shrsamba

import (
	"bytes"
	"context"
	"fmt"
	"text/template"

	"meoe.io/poly-csi/pkg/transport"
)

const shareTemplate = `
[{{ .ShareName }}]
  comment = poly-csi share {{ .ShareName }}
  path = {{ .Path }}
  read only = no
  browsable = no
  guest ok = no
  guest only = no
  valid users = {{ .User }}
  create mask = 0640
  directory mask = 0750
  force create mode = 0
  force directory mode = 0
  smb encrypt = required
`

type ShareParameters struct {
	Path      string
	User      string
}

func (sp ShareParameters) validate() error {
	if sp.Path == "" {
		return fmt.Errorf("share path missing")
	}
	if sp.User == "" {
		return fmt.Errorf("user name missing")
	}
	return nil
}

type snippets struct {
	tr transport.Transport
	gp globalParams
}

func newSambaSnippets(tr transport.Transport, gp globalParams) snippets {
	return snippets{
		tr: tr,
		gp: gp,
	}
}

func (s snippets) configureUser(ctx context.Context, name string, password string) error {
	cmd := append(createUserCmd, name)
	_, err := transport.RelaxedRun(ctx, s.tr, cmd, "already exists")
	if err != nil {
		return fmt.Errorf("create linux user: %w", err)
	}

	cmd = append(sambaSetPassCmd, name, password)
	err = transport.CheckedRun(ctx, s.tr, cmd)
	if err != nil {
		return fmt.Errorf("set samba password: %w", err)
	}
	return nil
}

func (s snippets) deleteUser(ctx context.Context, name string) error {
	cmd := append(sambaDeleteUserCmd, name)
	_, err := transport.RelaxedRun(ctx, s.tr, cmd, "does not exist")
	if err != nil {
		return fmt.Errorf("delete samba user: %w", err)
	}

	cmd = append(deleteUserCmd, name)
	_, err = transport.RelaxedRun(ctx, s.tr, cmd, "does not exist")
	if err != nil {
		return fmt.Errorf("delete linux user: %w", err)
	}
	return nil
}

func (s snippets) configureShare(ctx context.Context, name string, config ShareParameters) error {
	err := config.validate()
	if err != nil {
		return fmt.Errorf("validate share config: %w", err)
	}
	t, err := template.New("").Parse(shareTemplate)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	err = t.Execute(&buf, config)
	if err != nil {
		return err
	}

	configFile := fmt.Sprintf("%s/%s.conf", s.gp.sambaConfigDir, name)

	err = s.tr.SudoCmds().Write(ctx, configFile, buf.Bytes())
	if err != nil {
		return fmt.Errorf("write '%s': %w", configFile, err)
	}
	err = transport.CheckedRun(ctx, s.tr, sambaRefreshCmd)
	if err != nil {
		return fmt.Errorf("smb refresh: %w", err)
	}
	return nil
}

func (s snippets) removeShare(ctx context.Context, name string) error {
	configFile := fmt.Sprintf("%s/%s.conf", s.gp.sambaConfigDir, name)
	err := s.tr.SudoCmds().RmFile(ctx, configFile)
	if err != nil {
		return fmt.Errorf("remove '%s': %w", configFile, err)
	}
	err = transport.CheckedRun(ctx, s.tr, sambaRefreshCmd)
	if err != nil {
		return fmt.Errorf("smb refresh: %w", err)
	}
	return nil
}

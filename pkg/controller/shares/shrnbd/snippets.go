package shrnbd

import (
	"bytes"
	"context"
	"fmt"
	"text/template"

	"meoe.io/poly-csi/pkg/transport"
)

var nbdReloadCmd = []string{"nbd_reload"}

var exportTemplate = template.Must(template.New("").Parse(`
[{{ .ExportName }}]
exportname = "{{ .DevicePath }}"
# authfile = /export/nbd/export1-authfile
readonly = false
multifile = false
copyonwrite = false
flush = true
fua = true
trim = true
`))

type ShareConfig struct {
	ExportName string
	DevicePath string
}

type Snippets struct {
	tr transport.Transport
	gp globalParams
}

func newSnippets(tr transport.Transport, gp globalParams) Snippets {
	return Snippets{
		tr: tr,
		gp: gp,
	}
}

func (s *Snippets) configureShare(ctx context.Context, name string, share ShareConfig) error {
	file := fmt.Sprintf("%s/%s.conf", s.gp.configDir, name)

	var buf bytes.Buffer
	err := exportTemplate.Execute(&buf, share)
	if err != nil {
		return err
	}

	err = s.tr.SudoCmds().Write(ctx, file, buf.Bytes())
	if err != nil {
		return fmt.Errorf("write '%s': %w", file, err)
	}
	err = transport.CheckedRun(ctx, s.tr, nbdReloadCmd)
	if err != nil {
		return fmt.Errorf("nbd reload: %w", err)
	}
	return nil
}

func (s Snippets) removeShare(ctx context.Context, name string) error {
	file := fmt.Sprintf("%s/%s.conf", s.gp.configDir, name)

	err := s.tr.SudoCmds().RmFile(ctx, file)
	if err != nil {
		return fmt.Errorf("remove '%s': %w", file, err)
	}
	err = transport.CheckedRun(ctx, s.tr, nbdReloadCmd)
	if err != nil {
		return fmt.Errorf("nbd reload: %w", err)
	}
	return nil
}

package cntraoe

import (
	"context"
	"strings"

	"meoe.io/poly-csi/pkg/transport"
)

type Snippets struct {
	tr transport.Transport
}

func NewSnippets(tr transport.Transport) Snippets {
	return Snippets{
		tr: tr,
	}
}

func (s *Snippets) Discover(ctx context.Context) error {
	const discoverFile = "/dev/etherd/discover"
	return s.tr.Cmds().Write(ctx, discoverFile, []byte("\n"))
}

func (s *Snippets) Flush(ctx context.Context) error {
	const flushFile = "/dev/etherd/flush"
	return s.tr.Cmds().Write(ctx, flushFile, []byte("\n"))
}

func (s *Snippets) FlushDevice(ctx context.Context, devName string) error {
	const flushFile = "/dev/etherd/flush"
	return s.tr.Cmds().Write(ctx, flushFile, []byte(devName))
}

func (s *Snippets) RescanDevice(ctx context.Context, devName string) error {
	const revalidateFile = "/dev/etherd/revalidate"
	return s.tr.Cmds().Write(ctx, revalidateFile, []byte(devName))
}

func (s *Snippets) SetInterfaces(ctx context.Context, interfaces []string) error {
	const interfacesFile = "/dev/etherd/interfaces"
	return s.tr.Cmds().Write(ctx, interfacesFile, []byte(strings.Join(interfaces, " ")))
}

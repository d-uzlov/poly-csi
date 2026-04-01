package nvmetkeeper

import (
	"context"
	"fmt"

	"meoe.io/poly-csi/pkg/utils/nvmet"
)

func (k *NvmetKeeper) PortSubsysAttach(ctx context.Context, subsysNqn string, portName string) error {
	snip := nvmet.NewSnippets(ctx, k.tr, k.useSudo)

	err := snip.AttachSubsysToPort(subsysNqn, portName)
	if err != nil {
		return fmt.Errorf("AttachNqnToPort: %w", err)
	}

	err = k.str.saveSubsystemAttachment(ctx, subsysNqn, portName)
	if err != nil {
		return fmt.Errorf("saveSubsystemAttachment: %w", err)
	}
	return nil
}

func (k *NvmetKeeper) PortDetachSubsys(ctx context.Context, subsysNqn string, port string) error {
	snip := nvmet.NewSnippets(ctx, k.tr, k.useSudo)

	err := snip.DetachNqnFromPort(subsysNqn, port)
	if err != nil {
		return fmt.Errorf("DetachNqnFromPort: %w", err)
	}

	err = k.str.deleteSubsystemAttachment(ctx, subsysNqn, port)
	if err != nil {
		return fmt.Errorf("deleteSubsystemAttachment: %w", err)
	}
	return nil
}

func (k *NvmetKeeper) SubsysAllowHost(ctx context.Context, subsysNqn string, hostNqn string) error {
	snip := nvmet.NewSnippets(ctx, k.tr, k.useSudo)

	err := snip.AllowHost(subsysNqn, hostNqn)
	if err != nil {
		return fmt.Errorf("AllowHost: %w", err)
	}

	err = k.str.saveHostAttachment(ctx, subsysNqn, hostNqn)
	if err != nil {
		return fmt.Errorf("save attachment: %w", err)
	}
	return nil
}

func (k *NvmetKeeper) SubsysDisallowHost(ctx context.Context, subsysNqn string, hostNqn string) error {
	snip := nvmet.NewSnippets(ctx, k.tr, k.useSudo)

	err := snip.DisallowHost(subsysNqn, hostNqn)
	if err != nil {
		return fmt.Errorf("DisallowHost: %w", err)
	}

	err = k.str.deleteHostAttachment(ctx, subsysNqn, hostNqn)
	if err != nil {
		return fmt.Errorf("save attachment: %w", err)
	}
	return nil
}

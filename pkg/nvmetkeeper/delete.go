package nvmetkeeper

import (
	"context"
	"fmt"

	"meoe.io/poly-csi/pkg/utils/nvmet"
)

func (k *NvmetKeeper) PortDelete(ctx context.Context, portName string) error {
	snip := nvmet.NewSnippets(ctx, k.tr, k.useSudo)
	err := snip.DeletePort(portName)
	if err != nil {
		return fmt.Errorf("delete port: %w", err)
	}
	err = k.str.deletePortConfig(ctx, portName)
	if err != nil {
		return fmt.Errorf("delete port config: %w", err)
	}
	return nil
}

func (k *NvmetKeeper) SubsysDelete(ctx context.Context, subsysNqn string) error {
	snip := nvmet.NewSnippets(ctx, k.tr, k.useSudo)

	hostList, err := snip.ListHostAttachments(subsysNqn)
	if err != nil {
		return fmt.Errorf("list namespaces for '%s': %w", subsysNqn, err)
	}
	for _, v := range hostList {
		err = snip.DisallowHost(subsysNqn, v)
		if err != nil {
			return fmt.Errorf("delete host attachment '%s/%s': %w", subsysNqn, v, err)
		}
		err = k.str.deleteHostAttachment(ctx, subsysNqn, v)
		if err != nil {
			return fmt.Errorf("delete host attachment '%s/%s': %w", subsysNqn, v, err)
		}
	}

	nsList, err := snip.ListNamespaces(subsysNqn)
	if err != nil {
		return fmt.Errorf("list namespaces for '%s': %w", subsysNqn, err)
	}
	for _, v := range nsList {
		err = snip.DeleteNamespace(subsysNqn, v)
		if err != nil {
			return fmt.Errorf("delete namespace '%s/%s': %w", subsysNqn, v, err)
		}
		err = k.str.deleteNamespace(ctx, subsysNqn, v)
		if err != nil {
			return fmt.Errorf("delete namespace '%s/%s': %w", subsysNqn, v, err)
		}
	}

	err = snip.DeleteSubsys(subsysNqn)
	if err != nil {
		return fmt.Errorf("delete subsystem '%s': %w", subsysNqn, err)
	}
	err = k.str.deleteSubsystem(ctx, subsysNqn)
	if err != nil {
		return fmt.Errorf("delete subsys '%s': %w", subsysNqn, err)
	}

	return nil
}

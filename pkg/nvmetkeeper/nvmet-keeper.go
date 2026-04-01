package nvmetkeeper

import (
	"context"
	"fmt"

	"meoe.io/poly-csi/pkg/transport"
)

type NvmetKeeper struct {
	tr      transport.Transport
	useSudo bool
	str     ConfigStorage
}

func New(tr transport.Transport, configFolder string, useSudo bool, saveState bool) *NvmetKeeper {
	return &NvmetKeeper{
		tr:      tr,
		useSudo: useSudo,
		str:     NewConfigStorage(tr, configFolder, useSudo, saveState),
	}
}

func (k *NvmetKeeper) RestoreConfig(ctx context.Context) error {
	hosts, err := k.str.listHosts(ctx)
	if err != nil {
		return fmt.Errorf("list saved hosts: %w", err)
	}

	subsystems, err := k.str.listSubsystems(ctx)
	if err != nil {
		return fmt.Errorf("list saved subsystems: %w", err)
	}

	ports, err := k.str.listPorts(ctx)
	if err != nil {
		return fmt.Errorf("list saved ports: %w", err)
	}

	for _, v := range hosts {
		err = k.HostCreate(ctx, v)
		if err != nil {
			return fmt.Errorf("create host %s: %w", v.Nqn, err)
		}
	}

	for _, v := range subsystems {
		err = k.SubsysCreate(ctx, v.subsys)
		if err != nil {
			return fmt.Errorf("create subsys %s: %w", v.subsys.Nqn, err)
		}
		for _, n := range v.namespaces {
			err = k.NamespaceCreate(ctx, n)
			if err != nil {
				return fmt.Errorf("create namespace %s/%s: %w", n.Nqn, n.Nsid, err)
			}
		}
		for _, a := range v.attachedHosts {
			err = k.SubsysAllowHost(ctx, v.subsys.Nqn, a)
			if err != nil {
				return fmt.Errorf("allow host '%s' to access subsys %s: %w", a, v.subsys.Nqn, err)
			}
		}
	}

	// When storage server reboots, clients will periodically retry connections.
	// It's ok if storage server is not configured.
	// But if storage server is partially configured,
	// and ports are enabled, but without attached subsystems,
	// clients will receive "subsystem not found" error, and decide that subsystem is deleted, and will stop reconnection attempts.
	// Let's try to setup ports and subsys-port attachments as quickly as possible,
	// to reduce time when port is available without full list of subsystems.
	// This is a best-effort solution, for perfect race-free setup external network manipulations are required.

	for _, v := range ports {
		err = k.PortCreate(ctx, v)
		if err != nil {
			return fmt.Errorf("create port %s: %w", v.Name, err)
		}
	}

	for _, v := range subsystems {
		for _, a := range v.attachedPorts {
			err = k.PortSubsysAttach(ctx, v.subsys.Nqn, a)
			if err != nil {
				return fmt.Errorf("attach '%s' to port %s: %w", v.subsys.Nqn, a, err)
			}
		}
	}

	return nil
}

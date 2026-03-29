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
	ports, err := k.str.listPorts(ctx)
	if err != nil {
		return fmt.Errorf("list saved ports: %w", err)
	}
	for _, v := range ports {
		err = k.CreatePort(ctx, v)
		if err != nil {
			return fmt.Errorf("create port %s: %w", v.Name, err)
		}
	}

	hosts, err := k.str.listHosts(ctx)
	if err != nil {
		return fmt.Errorf("list saved hosts: %w", err)
	}
	for _, v := range hosts {
		err = k.CreateHost(ctx, v)
		if err != nil {
			return fmt.Errorf("create host %s: %w", v.Nqn, err)
		}
	}

	subsystems, err := k.str.listSubsystems(ctx)
	if err != nil {
		return fmt.Errorf("list saved subsystems: %w", err)
	}
	for _, v := range subsystems {
		err = k.CreateSubsystem(ctx, v.subsys)
		if err != nil {
			return fmt.Errorf("create subsys %s: %w", v.subsys.Nqn, err)
		}
		for _, n := range v.namespaces {
			err = k.CreateNamespace(ctx, n)
			if err != nil {
				return fmt.Errorf("create namespace %s/%s: %w", n.Nqn, n.Nsid, err)
			}
		}
		for _, a := range v.attachedPorts {
			err = k.AttachNqnToPort(ctx, v.subsys.Nqn, a)
			if err != nil {
				return fmt.Errorf("attach '%s' to port %s: %w", v.subsys.Nqn, a, err)
			}
		}
	}

	return nil
}

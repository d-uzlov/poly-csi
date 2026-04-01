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

func (k *NvmetKeeper) PortCreate(ctx context.Context, c nvmet.Port) error {
	snip := nvmet.NewSnippets(ctx, k.tr, k.useSudo)
	p, err := snip.ReadPort(c.Name)
	if err != nil {
		return fmt.Errorf("read existing port info: %w", err)
	}
	if p != nil {
		if *p != c {
			return fmt.Errorf("existing config does not match requested: existing %#v != requested %#v", *p, c)
		}
	} else {
		// port does not exist yet, need to create it
		err = snip.ConfigurePort(c)
		if err != nil {
			return fmt.Errorf("configure port: %w", err)
		}
	}
	err = k.str.savePort(ctx, c)
	if err != nil {
		return fmt.Errorf("save port: %w", err)
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

func (k *NvmetKeeper) SubsysCreate(ctx context.Context, s nvmet.Subsys) error {
	snip := nvmet.NewSnippets(ctx, k.tr, k.useSudo)

	existingSubsys, err := snip.ReadSubsys(s.Nqn)
	if err != nil {
		return fmt.Errorf("read existing subsys info: %w", err)
	}
	if existingSubsys != nil {
		if !nvmet.CheckSubsystemsEquivalent(s, *existingSubsys) {
			return fmt.Errorf("existing config does not match requested: existing %#v != requested %#v", *existingSubsys, s)
		}
	} else {
		// subsys does not exist yet, need to create it
		err = snip.ConfigureSubsys(s)
		if err != nil {
			return fmt.Errorf("configure subsys '%s': %w", s.Nqn, err)
		}
	}
	err = k.str.saveSubsystem(ctx, s)
	if err != nil {
		return fmt.Errorf("save port: %w", err)
	}

	return nil
}

func (k *NvmetKeeper) NamespaceCreate(ctx context.Context, n nvmet.Ns) error {
	snip := nvmet.NewSnippets(ctx, k.tr, k.useSudo)

	existingNs, err := snip.ReadNamespace(n.Nqn, n.Nsid)
	if err != nil {
		return fmt.Errorf("read existing subsys info: %w", err)
	}
	if existingNs != nil {
		if !nvmet.CheckNamespaceEquivalent(n, *existingNs) {
			return fmt.Errorf("existing config does not match requested: existing %#v != requested %#v", *existingNs, n)
		}
		if n.Enabled != existingNs.Enabled {
			err = snip.SetNamespaceEnabled(n)
			if err != nil {
				return fmt.Errorf("enable namespace '%s/%s': %w", n.Nqn, n.Nsid, err)
			}
		}
	} else {
		err = snip.ConfigureNamespace(n)
		if err != nil {
			return fmt.Errorf("configure namespace '%s/%s': %w", n.Nqn, n.Nsid, err)
		}
	}
	err = k.str.saveNamespace(ctx, n)
	if err != nil {
		return fmt.Errorf("save port: %w", err)
	}

	return nil
}

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

func (k *NvmetKeeper) HostCreate(ctx context.Context, n nvmet.Host) error {
	snip := nvmet.NewSnippets(ctx, k.tr, k.useSudo)

	existingHost, err := snip.ReadHost(n.Nqn)
	if err != nil {
		return fmt.Errorf("read existing subsys info: %w", err)
	}
	if existingHost != nil {
		if !existingHost.IsEqual(n) {
			// TODO remove secret keys from logs ??
			return fmt.Errorf("existing config does not match requested: existing %#v != requested %#v", *existingHost, n)
		}
	} else {
		// host does not exist yet, need to create it
		err = snip.ConfigureHost(n)
		if err != nil {
			return fmt.Errorf("configure host '%s': %w", n.Nqn, err)
		}
	}
	err = k.str.saveHost(ctx, n)
	if err != nil {
		return fmt.Errorf("save host: %w", err)
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

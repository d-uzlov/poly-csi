package nvmetkeeper

import (
	"context"
	"fmt"

	"meoe.io/poly-csi/pkg/utils/nvmet"
)

func (k *NvmetKeeper) DeletePort(ctx context.Context, name string) error {
	snip := nvmet.NewSnippets(ctx, k.tr, k.useSudo)
	err := snip.DeletePort(name)
	if err != nil {
		return fmt.Errorf("delete port: %w", err)
	}
	err = k.str.deletePortConfig(ctx, name)
	if err != nil {
		return fmt.Errorf("delete port config: %w", err)
	}
	return nil
}

func (k *NvmetKeeper) CreatePort(ctx context.Context, c nvmet.Port) error {
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

func (k *NvmetKeeper) DeleteSubsys(ctx context.Context, nqn string) error {
	snip := nvmet.NewSnippets(ctx, k.tr, k.useSudo)

	hostList, err := snip.ListHostAttachments(nqn)
	if err != nil {
		return fmt.Errorf("list namespaces for '%s': %w", nqn, err)
	}
	for _, v := range hostList {
		err = snip.DisallowHost(nqn, v)
		if err != nil {
			return fmt.Errorf("delete host attachment '%s/%s': %w", nqn, v, err)
		}
		err = k.str.deleteHostAttachment(ctx, nqn, v)
		if err != nil {
			return fmt.Errorf("delete host attachment '%s/%s': %w", nqn, v, err)
		}
	}

	nsList, err := snip.ListNamespaces(nqn)
	if err != nil {
		return fmt.Errorf("list namespaces for '%s': %w", nqn, err)
	}
	for _, v := range nsList {
		err = snip.DeleteNamespace(nqn, v)
		if err != nil {
			return fmt.Errorf("delete namespace '%s/%s': %w", nqn, v, err)
		}
		err = k.str.deleteNamespace(ctx, nqn, v)
		if err != nil {
			return fmt.Errorf("delete namespace '%s/%s': %w", nqn, v, err)
		}
	}

	err = snip.DeleteSubsys(nqn)
	if err != nil {
		return fmt.Errorf("delete subsystem '%s': %w", nqn, err)
	}
	err = k.str.deleteSubsystem(ctx, nqn)
	if err != nil {
		return fmt.Errorf("delete subsys '%s': %w", nqn, err)
	}

	return nil
}

func (k *NvmetKeeper) CreateSubsystem(ctx context.Context, s nvmet.Subsys) error {
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

func (k *NvmetKeeper) CreateNamespace(ctx context.Context, n nvmet.Ns) error {
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

func (k *NvmetKeeper) AttachNqnToPort(ctx context.Context, nqn string, port string) error {
	snip := nvmet.NewSnippets(ctx, k.tr, k.useSudo)

	err := snip.AttachSubsysToPort(nqn, port)
	if err != nil {
		return fmt.Errorf("AttachNqnToPort: %w", err)
	}

	err = k.str.saveSubsystemAttachment(ctx, nqn, port)
	if err != nil {
		return fmt.Errorf("saveSubsystemAttachment: %w", err)
	}
	return nil
}

func (k *NvmetKeeper) DetachPortFromNqn(ctx context.Context, nqn string, port string) error {
	snip := nvmet.NewSnippets(ctx, k.tr, k.useSudo)

	err := snip.DetachNqnFromPort(nqn, port)
	if err != nil {
		return fmt.Errorf("DetachNqnFromPort: %w", err)
	}

	err = k.str.deleteSubsystemAttachment(ctx, nqn, port)
	if err != nil {
		return fmt.Errorf("deleteSubsystemAttachment: %w", err)
	}
	return nil
}

func (k *NvmetKeeper) CreateHost(ctx context.Context, n nvmet.Host) error {
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

func (k *NvmetKeeper) AllowHost(ctx context.Context, nqn string, host string) error {
	snip := nvmet.NewSnippets(ctx, k.tr, k.useSudo)

	err := snip.AllowHost(nqn, host)
	if err != nil {
		return fmt.Errorf("AllowHost: %w", err)
	}

	err = k.str.saveHostAttachment(ctx, nqn, host)
	if err != nil {
		return fmt.Errorf("save attachment: %w", err)
	}
	return nil
}

func (k *NvmetKeeper) DisallowHost(ctx context.Context, nqn string, host string) error {
	snip := nvmet.NewSnippets(ctx, k.tr, k.useSudo)

	err := snip.DisallowHost(nqn, host)
	if err != nil {
		return fmt.Errorf("DisallowHost: %w", err)
	}

	err = k.str.deleteHostAttachment(ctx, nqn, host)
	if err != nil {
		return fmt.Errorf("save attachment: %w", err)
	}
	return nil
}

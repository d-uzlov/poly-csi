package nvmetkeeper

import (
	"context"
	"fmt"
	"io"

	"meoe.io/poly-csi/pkg/transport"
	"meoe.io/poly-csi/pkg/utils/nvmet"
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

func (k *NvmetKeeper) ResetState(ctx context.Context) error {
	snip := nvmet.NewSnippets(ctx, k.tr, k.useSudo)

	ports, err := snip.ListPorts()
	if err != nil {
		return fmt.Errorf("list ports: %w", err)
	}

	for _, v := range ports {
		attachments, err := snip.ListPortAttachments(v)
		if err != nil {
			return fmt.Errorf("list port %s subsys attachments: %w", v, err)
		}
		for _, a := range attachments {
			err = snip.DetachNqnFromPort(a, v)
			if err != nil {
				return fmt.Errorf("detach '%s' from port '%s': %w", a, v, err)
			}
		}

		err = snip.DeletePort(v)
		if err != nil {
			return fmt.Errorf("delete port '%s': %w", v, err)
		}
	}

	subsystems, err := snip.ListSubsys()
	if err != nil {
		return fmt.Errorf("list subsystems: %w", err)
	}

	for _, v := range subsystems {
		namespaces, err := snip.ListNamespaces(v)
		if err != nil {
			return fmt.Errorf("list namespaces: %w", err)
		}
		for _, nsid := range namespaces {
			err = snip.DeleteNamespace(v, nsid)
			if err != nil {
				return fmt.Errorf("delete namespace '%s/%s': %w", v, nsid, err)
			}
		}

		err = snip.DeleteSubsys(v)
		if err != nil {
			return fmt.Errorf("delete subsys '%s': %w", v, err)
		}
	}

	hosts, err := snip.ListHosts()
	if err != nil {
		return fmt.Errorf("list hosts: %w", err)
	}
	for _, v := range hosts {
		err = snip.DeleteHost(v)
		if err != nil {
			return fmt.Errorf("delete subsys '%s': %w", v, err)
		}
	}

	return nil
}

func (k *NvmetKeeper) ShowState(ctx context.Context, w io.Writer) error {
	snip := nvmet.NewSnippets(ctx, k.tr, k.useSudo)

	ports, err := snip.ListPorts()
	if err != nil {
		return fmt.Errorf("list ports: %w", err)
	}

	fmt.Fprintf(w, "ports:\n")
	for _, v := range ports {
		p, err := snip.ReadPort(v)
		if err != nil {
			return fmt.Errorf("read port %s: %w", v, err)
		}
		if p == nil {
			return fmt.Errorf("read port %s: not found", v)
		}
		fmt.Fprintf(w, "  %s: %#v\n", p.Name, p)

		attachments, err := snip.ListPortAttachments(v)
		if err != nil {
			return fmt.Errorf("list port %s subsys attachments: %w", v, err)
		}
		if len(attachments) != 0 {
			for _, a := range attachments {
				fmt.Fprintf(w, "    %s\n", a)
			}
		} else {
			fmt.Fprintf(w, "    no subsystems attached\n")
		}
	}

	hosts, err := snip.ListHosts()
	if err != nil {
		return fmt.Errorf("list hosts: %w", err)
	}

	fmt.Fprintf(w, "hosts:\n")
	for _, v := range hosts {
		p, err := snip.ReadHost(v)
		if err != nil {
			return fmt.Errorf("read host %s: %w", v, err)
		}
		if p == nil {
			return fmt.Errorf("read host %s: not found", v)
		}
		fmt.Fprintf(w, "  %s: auth=%v\n", p.Nqn, p.NeedAuth)
	}

	subsystems, err := snip.ListSubsys()
	if err != nil {
		return fmt.Errorf("list subsystems: %w", err)
	}

	fmt.Fprintf(w, "subsystems:\n")
	for _, v := range subsystems {
		s, err := snip.ReadSubsys(v)
		if err != nil {
			return fmt.Errorf("read subsys %s: %w", v, err)
		}
		if s == nil {
			return fmt.Errorf("read subsys %s: not found", v)
		}
		fmt.Fprintf(w, "  subsys %s: \n", s.Nqn)
		fmt.Fprintf(w, "    properties: allow_any_host=%v, serial=%s, model=%s\n", s.AllowAnyHost, s.Serial, s.Model)

		namespaces, err := snip.ListNamespaces(s.Nqn)
		if err != nil {
			return fmt.Errorf("list namespaces: %w", err)
		}

		if len(namespaces) != 0 {
			fmt.Fprintf(w, "    namespaces:\n")
			for _, nsid := range namespaces {
				n, err := snip.ReadNamespace(s.Nqn, nsid)
				if err != nil {
					return fmt.Errorf("read namespace %s: %w", nsid, err)
				}
				if n == nil {
					return fmt.Errorf("read namespace %s: not found", nsid)
				}
				fmt.Fprintf(w, "      %s: enabled=%v, device=%s uuid=%s\n", n.Nsid, n.Enabled, n.DevicePath, n.DeviceUuid)
			}
		} else {
			fmt.Fprintf(w, "    no namespaces configured\n")
		}

		subsysHosts, err := snip.ListHostAttachments(s.Nqn)
		if err != nil {
			return fmt.Errorf("list subsys host: %w", err)
		}
		if len(subsysHosts) != 0 {
			fmt.Fprintf(w, "    allowed hosts:\n")
			for _, h := range subsysHosts {
				fmt.Fprintf(w, "      %s\n", h)
			}
		}
	}

	return nil
}

func (k *NvmetKeeper) SaveStateToConfig(ctx context.Context) error {
	snip := nvmet.NewSnippets(ctx, k.tr, k.useSudo)

	ports, err := snip.ListPorts()
	if err != nil {
		return fmt.Errorf("list ports: %w", err)
	}
	for _, v := range ports {
		p, err := snip.ReadPort(v)
		if err != nil {
			return fmt.Errorf("read port %s: %w", v, err)
		}
		if p == nil {
			return fmt.Errorf("read port %s: not found", v)
		}
		err = k.str.savePort(ctx, *p)
		if err != nil {
			return fmt.Errorf("save port %s: %w", p.Name, err)
		}
	}

	hosts, err := snip.ListHosts()
	if err != nil {
		return fmt.Errorf("list hosts: %w", err)
	}
	for _, v := range hosts {
		p, err := snip.ReadHost(v)
		if err != nil {
			return fmt.Errorf("read host %s: %w", v, err)
		}
		if p == nil {
			return fmt.Errorf("read host %s: not found", v)
		}
		err = k.str.saveHost(ctx, *p)
		if err != nil {
			return fmt.Errorf("save host %s: %w", p.Nqn, err)
		}
	}

	subsystems, err := snip.ListSubsys()
	if err != nil {
		return fmt.Errorf("list subsystems: %w", err)
	}
	for _, v := range subsystems {
		s, err := snip.ReadSubsys(v)
		if err != nil {
			return fmt.Errorf("read subsys %s: %w", v, err)
		}
		if s == nil {
			return fmt.Errorf("read subsys %s: not found", v)
		}
		err = k.str.saveSubsystem(ctx, *s)
		if err != nil {
			return fmt.Errorf("save subsys %s: %w", s.Nqn, err)
		}

		namespaces, err := snip.ListNamespaces(s.Nqn)
		if err != nil {
			return fmt.Errorf("list namespaces: %w", err)
		}
		for _, v := range namespaces {
			n, err := snip.ReadNamespace(s.Nqn, v)
			if err != nil {
				return fmt.Errorf("read namespace %s: %w", v, err)
			}
			if n == nil {
				return fmt.Errorf("read namespace %s: not found", v)
			}
			err = k.str.saveNamespace(ctx, *n)
			if err != nil {
				return fmt.Errorf("save namespace %s: %w", n.Nqn, err)
			}
		}

		subsysHosts, err := snip.ListHostAttachments(s.Nqn)
		if err != nil {
			return fmt.Errorf("list subsys host: %w", err)
		}
		for _, h := range subsysHosts {
			err = k.str.saveHostAttachment(ctx, v, h)
			if err != nil {
				return fmt.Errorf("save host attachment '%s/hosts/%s': %w", v, h, err)
			}
		}
	}

	// we can't save attachments when first iterating over ports
	// because we need subsystems to be created first
	for _, v := range ports {
		attachments, err := snip.ListPortAttachments(v)
		if err != nil {
			return fmt.Errorf("list port %s subsys attachments: %w", v, err)
		}
		for _, a := range attachments {
			err = k.str.saveSubsystemAttachment(ctx, a, v)
			if err != nil {
				return fmt.Errorf("save subsys %s attachment to %s: %w", a, v, err)
			}
		}
	}

	return nil
}

func (k *NvmetKeeper) RestoreConfig(ctx context.Context) error {
	ports, err := k.str.listPorts(ctx)
	if err != nil {
		return fmt.Errorf("list saved configs: %w", err)
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

	err := snip.AttachNqnToPort(nqn, port)
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

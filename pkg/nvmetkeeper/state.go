package nvmetkeeper

import (
	"context"
	"fmt"
	"io"

	"meoe.io/poly-csi/pkg/utils/nvmet"
)

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

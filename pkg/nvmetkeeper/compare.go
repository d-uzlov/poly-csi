package nvmetkeeper

import (
	"context"
	"fmt"
	"io"

	"meoe.io/poly-csi/pkg/utils/nvmet"
)

type hostComparison struct {
	nqn    string
	state  *nvmet.Host
	config *nvmet.Host
}

func (k *NvmetKeeper) CompareHostState(ctx context.Context, w io.Writer) error {
	snip := nvmet.NewSnippets(ctx, k.tr, k.useSudo)

	stateHosts, err := snip.ListHosts()
	if err != nil {
		return fmt.Errorf("list hosts: %w", err)
	}
	configHosts, err := k.str.listHosts(ctx)
	if err != nil {
		return fmt.Errorf("list saved hosts: %w", err)
	}

	combined := map[string]hostComparison{}

	for _, v := range configHosts {
		val := combined[v.Nqn]
		val.config = &v
		val.nqn = v.Nqn
		combined[v.Nqn] = val
	}

	for _, v := range stateHosts {
		p, err := snip.ReadHost(v)
		if err != nil {
			return fmt.Errorf("read host %s: %w", v, err)
		}
		val := combined[v]
		val.state = p
		val.nqn = v
		combined[v] = val
	}

	fmt.Fprintf(w, "hosts:\n")
	changesCounter := 0

	for _, v := range combined {
		s := v.state
		c := v.config
		isChange := true
		switch {
		case c == nil:
			fmt.Fprintf(w, "  %s: state only\n", v.nqn)
		case s == nil:
			fmt.Fprintf(w, "  %s: config only\n", v.nqn)
		case c.NeedAuth != s.NeedAuth:
			fmt.Fprintf(w, "  %s: state auth='%v', config auth='%v'", v.nqn, s.NeedAuth, c.NeedAuth)
		default:
			var changes []string
			if c.DhGroup != s.DhGroup {
				changes = append(changes, fmt.Sprintf("dh_group: state='%s', config='%s'", s.DhGroup, c.DhGroup))
			}
			if c.HashType != s.HashType {
				changes = append(changes, fmt.Sprintf("hash_type: state='%s', config='%s'", s.HashType, c.HashType))
			}
			if c.HostKey != s.HostKey {
				changes = append(changes, fmt.Sprintf("host_key: state='%s', config='%s'", s.HostKey, c.HostKey))
			}
			if c.ControllerKey != s.ControllerKey {
				changes = append(changes, fmt.Sprintf("controller_key: state='%s', config='%s'", s.ControllerKey, c.ControllerKey))
			}
			if len(changes) == 0 {
				isChange = false
				// don't need to print anything
			} else if len(changes) == 1 {
				fmt.Fprintf(w, "  %s: %s\n", v.nqn, changes[0])
			} else {
				fmt.Fprintf(w, "  %s:\n", v.nqn)
				for _, v2 := range changes {
					fmt.Fprintf(w, "    %s\n", v2)
				}
			}
		}
		if isChange {
			changesCounter++
		}
	}

	if changesCounter == 0 {
		fmt.Fprintf(w, "  no changes\n")
	}

	return nil
}

type portComparison struct {
	name   string
	state  *nvmet.Port
	config *nvmet.Port
}

func (k *NvmetKeeper) ComparePortState(ctx context.Context, w io.Writer) error {
	snip := nvmet.NewSnippets(ctx, k.tr, k.useSudo)

	statePorts, err := snip.ListPorts()
	if err != nil {
		return fmt.Errorf("list ports: %w", err)
	}
	configPorts, err := k.str.listPorts(ctx)
	if err != nil {
		return fmt.Errorf("list saved ports: %w", err)
	}

	combined := map[string]portComparison{}

	for _, v := range configPorts {
		val := combined[v.Name]
		val.config = &v
		val.name = v.Name
		combined[v.Name] = val
	}

	for _, v := range statePorts {
		p, err := snip.ReadPort(v)
		if err != nil {
			return fmt.Errorf("read port %s: %w", v, err)
		}
		val := combined[v]
		val.state = p
		val.name = v
		combined[v] = val
	}

	fmt.Fprintf(w, "ports:\n")
	changesCounter := 0

	for _, v := range combined {
		s := v.state
		c := v.config
		isChange := true
		switch {
		case c == nil:
			fmt.Fprintf(w, "  %s: state only\n", v.name)
		case s == nil:
			fmt.Fprintf(w, "  %s: config only\n", v.name)
		default:
			var changes []string
			if c.TransportType != s.TransportType {
				changes = append(changes, fmt.Sprintf("transport_type: state='%s', config='%s'", s.TransportType, c.TransportType))
			}
			if c.AddressFamily != s.AddressFamily {
				changes = append(changes, fmt.Sprintf("address_family: state='%s', config='%s'", s.AddressFamily, c.AddressFamily))
			}
			if c.ListenAddr != s.ListenAddr {
				changes = append(changes, fmt.Sprintf("listen_address: state='%s', config='%s'", s.ListenAddr, c.ListenAddr))
			}
			if c.ListenPort != s.ListenPort {
				changes = append(changes, fmt.Sprintf("listen_port: state='%d', config='%d'", s.ListenPort, c.ListenPort))
			}
			if len(changes) == 0 {
				isChange = false
				// don't need to print anything
			} else if len(changes) == 1 {
				fmt.Fprintf(w, "  %s: %s\n", v.name, changes[0])
			} else {
				fmt.Fprintf(w, "  %s:\n", v.name)
				for _, v2 := range changes {
					fmt.Fprintf(w, "    %s\n", v2)
				}
			}
		}
		if isChange {
			changesCounter++
		}
	}

	if changesCounter == 0 {
		fmt.Fprintf(w, "  no changes\n")
	}

	return nil
}

type subsysComparison struct {
	nqn    string
	state  *savedSubsys
	config *savedSubsys
}

type namespaceComparison struct {
	nsid   string
	state  *nvmet.Ns
	config *nvmet.Ns
}

type linkComparison struct {
	name   string
	state  bool
	config bool
}

func (k *NvmetKeeper) CompareSubsysState(ctx context.Context, w io.Writer) error {
	snip := nvmet.NewSnippets(ctx, k.tr, k.useSudo)

	stateSubsys, err := snip.ListSubsys()
	if err != nil {
		return fmt.Errorf("list subsystems: %w", err)
	}
	configSubsys, err := k.str.listSubsystems(ctx)
	if err != nil {
		return fmt.Errorf("list saved subsystems: %w", err)
	}

	combined := map[string]subsysComparison{}

	for _, v := range configSubsys {
		val := combined[v.subsys.Nqn]
		val.config = &v
		val.nqn = v.subsys.Nqn
		combined[v.subsys.Nqn] = val
	}

	for _, v := range stateSubsys {
		subsys, err := snip.ReadSubsys(v)
		if err != nil {
			return fmt.Errorf("read subsys %s: %w", v, err)
		}
		if subsys == nil {
			continue
		}
		nsIds, err := snip.ListNamespaces(subsys.Nqn)
		if err != nil {
			return fmt.Errorf("read namespaces for %s: %w", v, err)
		}
		var nsList []nvmet.Ns
		for _, n := range nsIds {
			ns, err := snip.ReadNamespace(subsys.Nqn, n)
			if err != nil {
				return fmt.Errorf("read namespace %s/%s", subsys.Nqn, n)
			}
			if ns == nil {
				continue
			}
			nsList = append(nsList, *ns)
		}
		hosts, err := snip.ListHostAttachments(subsys.Nqn)
		if err != nil {
			return fmt.Errorf("read hosts for %s: %w", v, err)
		}
		fullSubsys := savedSubsys{
			subsys:     *subsys,
			namespaces: nsList,
			attachedHosts: hosts,
		}
		val := combined[v]
		val.state = &fullSubsys
		val.nqn = v
		combined[v] = val
	}

	// fill subsys-port relationship from ports
	statePorts, err := snip.ListPorts()
	if err != nil {
		return fmt.Errorf("list ports: %w", err)
	}
	for _, p := range statePorts {
		subsystems, err := snip.ListPortAttachments(p)
		if err != nil {
			return fmt.Errorf("read port %s attachments: %w", p, err)
		}
		for _, s := range subsystems {
			val := combined[s]
			if val.state == nil {
				val.state = &savedSubsys{}
			}
			val.state.attachedPorts = append(val.state.attachedPorts, p)
			val.nqn = s
			combined[p] = val
		}
	}

	fmt.Fprintf(w, "subsystems:\n")
	changesCounter := 0

	for _, v := range combined {
		s := v.state
		c := v.config
		isChange := true
		switch {
		case c == nil:
			fmt.Fprintf(w, "  %s: state only\n", v.nqn)
		case s == nil:
			fmt.Fprintf(w, "  %s: config only\n", v.nqn)
		default:
			var changes []string
			if c.subsys.AllowAnyHost != s.subsys.AllowAnyHost {
				changes = append(changes, fmt.Sprintf("allow_any_host: state='%v', config='%v'", s.subsys.AllowAnyHost, c.subsys.AllowAnyHost))
			}
			if c.subsys.Model != s.subsys.Model {
				changes = append(changes, fmt.Sprintf("model: state='%s', config='%s'", s.subsys.Model, c.subsys.Model))
			}
			if c.subsys.Serial != s.subsys.Serial {
				changes = append(changes, fmt.Sprintf("serial: state='%s', config='%s'", s.subsys.Serial, c.subsys.Serial))
			}

			combinedNs := map[string]namespaceComparison{}

			for _, v := range c.namespaces {
				val := combinedNs[v.Nsid]
				val.config = &v
				val.nsid = v.Nsid
				combinedNs[v.Nsid] = val
			}

			for _, v := range s.namespaces {
				val := combinedNs[v.Nsid]
				val.state = &v
				val.nsid = v.Nsid
				combinedNs[v.Nsid] = val
			}

			for _, v := range combinedNs {
				switch {
				case v.config == nil:
					changes = append(changes, fmt.Sprintf("ns %s: state only", v.nsid))
				case v.state == nil:
					changes = append(changes, fmt.Sprintf("ns %s: config only", v.nsid))
				default:
					if v.state.DevicePath != v.config.DevicePath {
						changes = append(changes, fmt.Sprintf("ns %s: device_path: state='%s', config='%s'", v.nsid, v.state.DevicePath, v.config.DevicePath))
					}
					if v.state.DeviceGuid != v.config.DeviceGuid {
						changes = append(changes, fmt.Sprintf("ns %s: device_guid: state='%s', config='%s'", v.nsid, v.state.DeviceGuid, v.config.DeviceGuid))
					}
					if v.state.DeviceUuid != v.config.DeviceUuid {
						changes = append(changes, fmt.Sprintf("ns %s: device_uuid: state='%s', config='%s'", v.nsid, v.state.DeviceUuid, v.config.DeviceUuid))
					}
				}
			}

			combinedPorts := map[string]linkComparison{}
			for _, v := range c.attachedPorts {
				val := combinedPorts[v]
				val.config = true
				val.name = v
				combinedPorts[v] = val
			}

			for _, v := range s.attachedPorts {
				val := combinedPorts[v]
				val.state = true
				val.name = v
				combinedPorts[v] = val
			}

			for _, v := range combinedPorts {
				switch {
				case !v.config:
					changes = append(changes, fmt.Sprintf("port %s: state only", v.name))
				case !v.state:
					changes = append(changes, fmt.Sprintf("port %s: config only", v.name))
				default:
					// nothing to do
				}
			}

			combinedHosts := map[string]linkComparison{}
			for _, v := range c.attachedHosts {
				val := combinedHosts[v]
				val.config = true
				val.name = v
				combinedHosts[v] = val
			}

			for _, v := range s.attachedHosts {
				val := combinedHosts[v]
				val.state = true
				val.name = v
				combinedHosts[v] = val
			}

			for _, v := range combinedHosts {
				switch {
				case !v.config:
					changes = append(changes, fmt.Sprintf("host %s: state only", v.name))
				case !v.state:
					changes = append(changes, fmt.Sprintf("host %s: config only", v.name))
				default:
					// nothing to do
				}
			}

			if len(changes) == 0 {
				isChange = false
				// don't need to print anything
			} else if len(changes) == 1 {
				fmt.Fprintf(w, "  %s: %s\n", v.nqn, changes[0])
			} else {
				fmt.Fprintf(w, "  %s:\n", v.nqn)
				for _, v2 := range changes {
					fmt.Fprintf(w, "    %s\n", v2)
				}
			}
		}
		if isChange {
			changesCounter++
		}
	}

	if changesCounter == 0 {
		fmt.Fprintf(w, "  no changes\n")
	}

	return nil
}

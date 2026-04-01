package nvmetkeeper

import (
	"context"
	"fmt"

	"meoe.io/poly-csi/pkg/utils/nvmet"
)

func (k *NvmetKeeper) PortCreate(ctx context.Context, req nvmet.Port) error {
	snip := nvmet.NewSnippets(ctx, k.tr, k.useSudo)
	existing, err := snip.ReadPort(req.Name)
	if err != nil {
		return fmt.Errorf("read port from state: %w", err)
	}
	if existing == nil {
		err = snip.ConfigurePort(req)
		if err != nil {
			return fmt.Errorf("configure port: %w", err)
		}
		existing, err = snip.ReadPort(req.Name)
		if err != nil {
			return fmt.Errorf("re-read port from state: %w", err)
		}
	}

	if *existing != req {
		return fmt.Errorf("state does not match requested value: existing %#v != requested %#v", *existing, req)
	}

	err = k.str.savePort(ctx, *existing)
	if err != nil {
		return fmt.Errorf("save port: %w", err)
	}
	return nil
}

func (k *NvmetKeeper) SubsysCreate(ctx context.Context, req nvmet.Subsys) error {
	snip := nvmet.NewSnippets(ctx, k.tr, k.useSudo)

	existing, err := snip.ReadSubsys(req.Nqn)
	if err != nil {
		return fmt.Errorf("read subsys from state: %w", err)
	}
	if existing == nil {
		err = snip.ConfigureSubsys(req)
		if err != nil {
			return fmt.Errorf("configure subsys '%s': %w", req.Nqn, err)
		}
		existing, err = snip.ReadSubsys(req.Nqn)
		if err != nil {
			return fmt.Errorf("re-read subsys from state: %w", err)
		}
	}

	if !nvmet.CheckSubsystemsEquivalent(req, *existing) {
		return fmt.Errorf("state does not match requested value: existing %#v != requested %#v", *existing, req)
	}

	err = k.str.saveSubsystem(ctx, *existing)
	if err != nil {
		return fmt.Errorf("save port: %w", err)
	}

	return nil
}

func (k *NvmetKeeper) NamespaceCreate(ctx context.Context, req nvmet.Ns) error {
	snip := nvmet.NewSnippets(ctx, k.tr, k.useSudo)

	existing, err := snip.ReadNamespace(req.Nqn, req.Nsid)
	if err != nil {
		return fmt.Errorf("read namespace from state: %w", err)
	}
	if existing == nil {
		err = snip.ConfigureNamespace(req)
		if err != nil {
			return fmt.Errorf("configure namespace '%s/%s': %w", req.Nqn, req.Nsid, err)
		}
		existing, err = snip.ReadNamespace(req.Nqn, req.Nsid)
		if err != nil {
			return fmt.Errorf("re-read namespace from state: %w", err)
		}
	}

	if !nvmet.CheckNamespaceEquivalent(req, *existing) {
		return fmt.Errorf("state does not match requested value: existing %#v != requested %#v", *existing, req)
	}
	if req.Enabled != existing.Enabled {
		err = snip.SetNamespaceEnabled(req)
		if err != nil {
			return fmt.Errorf("enable namespace '%s/%s': %w", req.Nqn, req.Nsid, err)
		}
		existing.Enabled = req.Enabled
	}

	err = k.str.saveNamespace(ctx, *existing)
	if err != nil {
		return fmt.Errorf("save port: %w", err)
	}

	return nil
}

func (k *NvmetKeeper) HostCreate(ctx context.Context, req nvmet.Host) error {
	snip := nvmet.NewSnippets(ctx, k.tr, k.useSudo)

	existing, err := snip.ReadHost(req.Nqn)
	if err != nil {
		return fmt.Errorf("read host from state: %w", err)
	}
	if existing == nil {
		err = snip.ConfigureHost(req)
		if err != nil {
			return fmt.Errorf("configure host '%s': %w", req.Nqn, err)
		}
		existing, err = snip.ReadHost(req.Nqn)
		if err != nil {
			return fmt.Errorf("re-read host from state: %w", err)
		}
	}

	if !existing.IsEqual(req) {
		// TODO remove secret keys from logs ??
		return fmt.Errorf("state does not match requested value: existing %#v != requested %#v", *existing, req)
	}

	err = k.str.saveHost(ctx, *existing)
	if err != nil {
		return fmt.Errorf("save host: %w", err)
	}

	return nil
}

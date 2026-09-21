package cntriscsiadm

import (
	"context"
	"fmt"
	"strings"

	"meoe.io/poly-csi/pkg/transport"
)

type iscsiSnippets struct {
	tr          transport.Transport
	iscsiadmCmd []string
}

func newIscsiSnippets(tr transport.Transport, iscsiadmCmd []string) iscsiSnippets {
	return iscsiSnippets{
		tr:          tr,
		iscsiadmCmd: iscsiadmCmd,
	}
}

// creates an entry in iscsi database for remote target and sets attributes
func (s iscsiSnippets) configureTarget(ctx context.Context, iqn string, server string, attributes map[string]string) error {
	cmd := append(s.iscsiadmCmd, "--mode=node", "--targetname", iqn, "--portal", server, "--op=new")
	err := transport.CheckedRun(ctx, s.tr, cmd)
	if err != nil {
		return fmt.Errorf("add target '%s@%s' to iscsiadm local db: %w", iqn, server, err)
	}
	for k, v := range attributes {
		cmd = append(s.iscsiadmCmd, "--mode=node", "--targetname", iqn, "--portal", server, "--op=update", "--name", k, "--value", v)
		err = transport.CheckedRun(ctx, s.tr, cmd)
		if err != nil {
			return fmt.Errorf("target '%s@%s' attribute '%s': %w", iqn, server, k, err)
		}
	}
	return nil
}

// connects to target
// target must already exist in the local iscsi database, see configureTarget()
func (s iscsiSnippets) connect(ctx context.Context, iqn string, server string) error {
	cmd := append(s.iscsiadmCmd, "--mode=node", "--targetname", iqn, "--portal", server, "--login")
	_, err := transport.RelaxedRun(ctx, s.tr, cmd, "1 session requested, but 1 already present")
	if err != nil {
		return fmt.Errorf("iscsiadm login: %w", err)
	}
	return nil
}

func (s iscsiSnippets) disconnect(ctx context.Context, iqn string, server string) error {
	cmd := append(s.iscsiadmCmd, "--mode=node", "--targetname", iqn, "--portal", server, "--logout")
	_, err := transport.RelaxedRun(ctx, s.tr, cmd, "No matching sessions found")
	if err != nil {
		return fmt.Errorf("iscsiadm logout: %w", err)
	}
	// do we need to also run `iscsiadm --op delete` ?
	// it doesn't seems like it's necessary
	return nil
}

type iscsiSession struct {
	protocol string
	index    string
	address  string
	tpg      string
	iqn      string
}

func (s iscsiSnippets) findSessions(ctx context.Context, iqn string) ([]iscsiSession, error) {
	allSessions, err := s.listSessions(ctx)
	if err != nil {
		return nil, err
	}
	var result []iscsiSession
	for _, v := range allSessions {
		if v.iqn == iqn {
			result = append(result, v)
		}
	}
	return result, nil
}

func (s iscsiSnippets) listSessions(ctx context.Context) ([]iscsiSession, error) {
	cmd := append(s.iscsiadmCmd, "--mode=session")
	stdout, err := transport.CheckedRunStdout(ctx, s.tr, cmd)
	if err != nil {
		return nil, fmt.Errorf("iscsiadm list sessions: %w", err)
	}
	var result []iscsiSession
	for line := range strings.Lines(stdout) {
		f := strings.Fields(line)
		if len(f) < 4 {
			return nil, fmt.Errorf("parse session '%s': invalid format: expected 4 fields", line)
		}
		s := iscsiSession{}
		s.protocol = f[0]
		s.protocol = strings.TrimSuffix(s.index, ":")
		s.index = f[1]
		s.index = strings.TrimPrefix(s.index, "[")
		s.index = strings.TrimSuffix(s.index, "]")
		addr := strings.Split(f[2], ",")
		if len(addr) < 2 {
			return nil, fmt.Errorf("parse session '%s': invalid address format: '%s'", line, f[2])
		}
		s.address = addr[0]
		s.tpg = addr[1]
		s.iqn = f[3]
		result = append(result, s)
	}
	return result, nil
}

func (s iscsiSnippets) rescanAll(ctx context.Context) error {
	cmd := append(s.iscsiadmCmd, "--mode=session", "--rescan")
	err := transport.CheckedRun(ctx, s.tr, cmd)
	if err != nil {
		return fmt.Errorf("iscsiadm rescan: %w", err)
	}
	return nil
}

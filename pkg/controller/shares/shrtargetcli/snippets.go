package shrtargetcli

import (
	"context"
	"fmt"
	"strings"

	"meoe.io/poly-csi/pkg/node/connectors/cntriscsiadm"
	"meoe.io/poly-csi/pkg/transport"
)

type targetcliSnippets struct {
	tr transport.Transport

	targetcliCmd []string
}

func newTargetcliSnippets(tr transport.Transport, targetcliCmd []string) targetcliSnippets {
	return targetcliSnippets{
		tr:           tr,
		targetcliCmd: targetcliCmd,
	}
}

func (s targetcliSnippets) save(ctx context.Context) error {
	cmd := append(s.targetcliCmd, "/", "saveconfig")
	err := transport.CheckedRun(ctx, s.tr, cmd)
	if err != nil {
		return fmt.Errorf("%s: %w", cmd, err)
	}
	return nil
}

func (s targetcliSnippets) configureBackstore(ctx context.Context, name string, device string) error {
	cmd := append(s.targetcliCmd, "/backstores/block", "create", name, device)
	_, err := transport.RelaxedRun(ctx, s.tr, cmd, "Storage object block/test-zvol exists")
	if err != nil {
		return fmt.Errorf("%s: %w", cmd, err)
	}
	cmd = append(s.targetcliCmd, "/backstores/block/"+name, "set", "attribute", "emulate_tpu=1")
	err = transport.CheckedRun(ctx, s.tr, cmd)
	if err != nil {
		return fmt.Errorf("%s: %w", cmd, err)
	}
	return nil
}

func (s targetcliSnippets) deleteBackstore(ctx context.Context, name string) error {
	cmd := append(s.targetcliCmd, "/backstores/block", "delete", name)
	_, err := transport.RelaxedRun(ctx, s.tr, cmd, "No storage object named")
	if err != nil {
		return fmt.Errorf("%s: %w", cmd, err)
	}
	return nil
}

type iscsiTargetConfig struct {
	iqn           string
	backstoreName string
	auth          bool
	demo          bool
	// user/pass should be set per-client, not on target. Delete?
	user string
	pass string
}

func (s targetcliSnippets) readAttribute(ctx context.Context, path string, group string, name string) (string, error) {
	cmd := append(s.targetcliCmd, path, "get", group, name)
	runResult, err := s.tr.RunCommand(ctx, cmd)
	if err != nil {
		return "", err
	}
	if runResult.ExitCode != 0 {
		if strings.Contains(string(runResult.Stderr), "No such path") {
			return "", nil
		}
		err = transport.FormatRunError(runResult)
		return "", fmt.Errorf("%s: %w", cmd, err)
	}

	value, hasPrefix := strings.CutPrefix(strings.TrimSpace(string(runResult.Stdout)), name+"=")
	if !hasPrefix || value == "" {
		return "", nil
	}
	return value, nil
}

func (s targetcliSnippets) configureTarget(ctx context.Context, subsysConfig iscsiTargetConfig) error {
	cmd := append(s.targetcliCmd, "/iscsi", "create", subsysConfig.iqn)
	_, err := transport.RelaxedRun(ctx, s.tr, cmd, "This Target already exists in configFS")
	if err != nil {
		return fmt.Errorf("%s: %w", cmd, err)
	}

	auth := "0"
	if subsysConfig.auth {
		auth = "1"
	}

	tpgPath := fmt.Sprintf("/iscsi/%s/tpg1", subsysConfig.iqn)
	cmd = append(s.targetcliCmd, tpgPath, "set", "attribute", "authentication="+auth)
	err = transport.CheckedRun(ctx, s.tr, cmd)
	if err != nil {
		return fmt.Errorf("%s: %w", cmd, err)
	}

	generateAcls := "0"
	if subsysConfig.demo {
		generateAcls = "1"
	}
	cmd = append(s.targetcliCmd, tpgPath, "set", "attribute", "generate_node_acls="+generateAcls)
	err = transport.CheckedRun(ctx, s.tr, cmd)
	if err != nil {
		return fmt.Errorf("%s: %w", cmd, err)
	}
	cmd = append(s.targetcliCmd, tpgPath, "set", "attribute", "demo_mode_write_protect=0")
	err = transport.CheckedRun(ctx, s.tr, cmd)
	if err != nil {
		return fmt.Errorf("%s: %w", cmd, err)
	}
	cmd = append(s.targetcliCmd, tpgPath, "set", "attribute", "cache_dynamic_acls=1")
	err = transport.CheckedRun(ctx, s.tr, cmd)
	if err != nil {
		return fmt.Errorf("%s: %w", cmd, err)
	}

	cmd = append(s.targetcliCmd, tpgPath+"/luns", "create", "/backstores/block/"+subsysConfig.backstoreName)
	_, err = transport.RelaxedRun(ctx, s.tr, cmd, "already exists")
	if err != nil {
		return err
	}

	if subsysConfig.user != "" && subsysConfig.pass != "" {
		cmd = append(s.targetcliCmd, tpgPath, "set", "auth", "userid="+subsysConfig.user)
		err = transport.CheckedRun(ctx, s.tr, cmd)
		if err != nil {
			return fmt.Errorf("%s '/iscsi/%s/tpg1' set auth userid: %w", s.targetcliCmd, subsysConfig.iqn, err)
		}
		cmd = append(s.targetcliCmd, tpgPath, "set", "auth", "password="+subsysConfig.pass)
		err = transport.CheckedRun(ctx, s.tr, cmd)
		if err != nil {
			return fmt.Errorf("%s '/iscsi/%s/tpg1' set auth password: %w", s.targetcliCmd, subsysConfig.iqn, err)
		}
	}

	return nil
}

func (s targetcliSnippets) deleteTarget(ctx context.Context, subsysConfig iscsiTargetConfig) error {
	cmd := append(s.targetcliCmd, "/iscsi", "delete", subsysConfig.iqn)
	_, err := transport.RelaxedRun(ctx, s.tr, cmd, "No such Target in configfs")
	if err != nil {
		return fmt.Errorf("%s: %w", cmd, err)
	}
	return nil
}

type chapAcl struct {
	client     cntriscsiadm.Chap
	controller cntriscsiadm.Chap
}

func (s targetcliSnippets) readClientAcl(ctx context.Context, targetIqn string, clientIqn string) (*chapAcl, error) {
	path := fmt.Sprintf("/iscsi/%s/tpg1/acls/%s", targetIqn, clientIqn)
	user, err := s.readAttribute(ctx, path, "auth", "userid")
	if err != nil {
		return nil, err
	}
	if user == "" {
		return nil, nil
	}
	pass, err := s.readAttribute(ctx, path, "auth", "password")
	if err != nil {
		return nil, err
	}
	if pass == "" {
		return nil, nil
	}
	client := cntriscsiadm.Chap{User: user, Pass: pass}

	mutualUser, err := s.readAttribute(ctx, path, "auth", "mutual_userid")
	if err != nil {
		return nil, err
	}
	if mutualUser == "" {
		return nil, nil
	}
	mutualPass, err := s.readAttribute(ctx, path, "auth", "mutual_password")
	if err != nil {
		return nil, err
	}
	if mutualPass == "" {
		return nil, nil
	}
	controller := cntriscsiadm.Chap{User: mutualUser, Pass: mutualPass}

	return &chapAcl{
		client:     client,
		controller: controller,
	}, nil
}

func (s targetcliSnippets) createClient(ctx context.Context, targetIqn string, clientIqn string) error {
	cmd := append(s.targetcliCmd, fmt.Sprintf("/iscsi/%s/tpg1/acls", targetIqn), "create", clientIqn)
	_, err := transport.RelaxedRun(ctx, s.tr, cmd, "This NodeACL already exists in configFS")
	if err != nil {
		return fmt.Errorf("%s: %w", cmd, err)
	}
	return nil
}

func (s targetcliSnippets) setupClientAcl(ctx context.Context, targetIqn string, clientIqn string, acl chapAcl) error {
	path := fmt.Sprintf("/iscsi/%s/tpg1/acls/%s", targetIqn, clientIqn)
	cmd := append(s.targetcliCmd, path, "set", "auth", "userid="+acl.client.User)
	err := transport.CheckedRun(ctx, s.tr, cmd)
	if err != nil {
		return fmt.Errorf("%s '%s' set auth userid: %w", s.targetcliCmd, path, err)
	}
	cmd = append(s.targetcliCmd, path, "set", "auth", "password="+acl.client.Pass)
	err = transport.CheckedRun(ctx, s.tr, cmd)
	if err != nil {
		return fmt.Errorf("%s '%s' set auth password: %w", s.targetcliCmd, path, err)
	}
	cmd = append(s.targetcliCmd, path, "set", "auth", "mutual_userid="+acl.controller.User)
	err = transport.CheckedRun(ctx, s.tr, cmd)
	if err != nil {
		return fmt.Errorf("%s '%s' set auth mutual_userid: %w", s.targetcliCmd, path, err)
	}
	cmd = append(s.targetcliCmd, path, "set", "auth", "mutual_password="+acl.controller.Pass)
	err = transport.CheckedRun(ctx, s.tr, cmd)
	if err != nil {
		return fmt.Errorf("%s '%s' set auth mutual_password: %w", s.targetcliCmd, path, err)
	}
	return nil
}

func (s targetcliSnippets) deleteClientAcl(ctx context.Context, targetIqn string, clientIqn string) error {
	path := fmt.Sprintf("/iscsi/%s/tpg1/acls", targetIqn)
	cmd := append(s.targetcliCmd, path, "delete", clientIqn)
	_, err := transport.RelaxedRun(ctx, s.tr, cmd, "No such NodeACL in configfs")
	if err != nil {
		return fmt.Errorf("%s: %w", cmd, err)
	}
	return nil
}

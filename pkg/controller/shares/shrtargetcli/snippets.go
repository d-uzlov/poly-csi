package shrtargetcli

import (
	"context"
	"fmt"
	"strings"

	"meoe.io/poly-csi/pkg/transport"
)

var targetcliCmd = []string{"targetcli"}

type targetcliSnippets struct {
	tr transport.Transport
}

func newTargetcliSnippets(tr transport.Transport) targetcliSnippets {
	return targetcliSnippets{
		tr: tr,
	}
}

func (s targetcliSnippets) save(ctx context.Context) error {
	cmd := append(targetcliCmd, "/", "saveconfig")
	err := transport.CheckedRun(ctx, s.tr, cmd, transport.WithSudo(true))
	if err != nil {
		// no need for annotations
		return err
	}
	return nil
}

func (s targetcliSnippets) configureBackstore(ctx context.Context, name string, device string) error {
	cmd := append(targetcliCmd, "/backstores/block", "create", name, device)
	_, err := transport.RelaxedRun(ctx, s.tr, cmd, "Storage object block/test-zvol exists", transport.WithSudo(true))
	if err != nil {
		return fmt.Errorf("create backstore '%s' for '%s': %w", name, device, err)
	}
	err = s.setAttribute(ctx, "/backstores/block/"+name, "attribute", "emulate_tpu", "1")
	if err != nil {
		// no need for annotations
		return err
	}
	return nil
}

func (s targetcliSnippets) deleteBackstore(ctx context.Context, name string) error {
	cmd := append(targetcliCmd, "/backstores/block", "delete", name)
	_, err := transport.RelaxedRun(ctx, s.tr, cmd, "No storage object named", transport.WithSudo(true))
	if err != nil {
		return fmt.Errorf("delete backstore '%s': %w", name, err)
	}
	return nil
}

type iscsiTargetConfig struct {
	iqn           string
	backstoreName string
	auth          bool
	demo          bool
}

func (s targetcliSnippets) readAttribute(ctx context.Context, path string, group string, name string) (string, error) {
	cmd := append(targetcliCmd, path, "get", group, name)
	runResult, err := s.tr.RunCommand(ctx, cmd, transport.WithSudo(true))
	if err != nil {
		return "", err
	}
	if runResult.ExitCode != 0 {
		if strings.Contains(string(runResult.Stderr), "No such path") {
			return "", nil
		}
		err = transport.FormatRunError(runResult)
		return "", fmt.Errorf("read %s/%s at '%s': %w", group, name, path, err)
	}

	value, hasPrefix := strings.CutPrefix(strings.TrimSpace(string(runResult.Stdout)), name+"=")
	if !hasPrefix || value == "" {
		return "", nil
	}
	return value, nil
}

func (s targetcliSnippets) setAttribute(ctx context.Context, path string, group string, name string, value string) error {
	cmd := append(targetcliCmd, path, "set", group, fmt.Sprintf("%s=%s", name, value))
	err := transport.CheckedRun(ctx, s.tr, cmd, transport.WithSudo(true))
	if err != nil {
		return fmt.Errorf("set %s/%s='%s' at '%s': %w", group, name, value, path, err)
	}
	return nil
}

func (s targetcliSnippets) setAttributes(ctx context.Context, path string, group string, values map[string]string) error {
	for k, v := range values {
		err := s.setAttribute(ctx, path, group, k, v)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s targetcliSnippets) configureTarget(ctx context.Context, subsysConfig iscsiTargetConfig) error {
	cmd := append(targetcliCmd, "/iscsi", "create", subsysConfig.iqn)
	_, err := transport.RelaxedRun(ctx, s.tr, cmd, "This Target already exists in configFS", transport.WithSudo(true))
	if err != nil {
		return fmt.Errorf("create target '%s': %w", subsysConfig.iqn, err)
	}

	tpgAttributes := map[string]string{
		"authentication":          "0",
		"generate_node_acls":      "0",
		"demo_mode_write_protect": "0",
		"cache_dynamic_acls":      "1",
	}

	if subsysConfig.auth {
		tpgAttributes["authentication"] = "1"
	}
	if subsysConfig.demo {
		tpgAttributes["generate_node_acls"] = "1"
	}

	tpgPath := fmt.Sprintf("/iscsi/%s/tpg1", subsysConfig.iqn)

	err = s.setAttributes(ctx, tpgPath, "attribute", tpgAttributes)
	if err != nil {
		return fmt.Errorf("configure tpg attributes: %w", err)
	}

	cmd = append(targetcliCmd, tpgPath+"/luns", "create", "/backstores/block/"+subsysConfig.backstoreName)
	_, err = transport.RelaxedRun(ctx, s.tr, cmd, "already exists", transport.WithSudo(true))
	if err != nil {
		return fmt.Errorf("connect backstore '%s' to '%s': %w", subsysConfig.backstoreName, tpgPath, err)
	}

	return nil
}

func (s targetcliSnippets) deleteTarget(ctx context.Context, iqn string) error {
	cmd := append(targetcliCmd, "/iscsi", "delete", iqn)
	_, err := transport.RelaxedRun(ctx, s.tr, cmd, "No such Target in configfs", transport.WithSudo(true))
	if err != nil {
		return fmt.Errorf("delete target '%s': %w", iqn, err)
	}
	return nil
}

type chapAcl struct {
	userId         string
	password       string
	mutualUserId   string
	mutualPassword string
}

func (s targetcliSnippets) readClientAcl(ctx context.Context, targetIqn string, clientIqn string) (*chapAcl, error) {
	result := chapAcl{}
	var err error

	path := fmt.Sprintf("/iscsi/%s/tpg1/acls/%s", targetIqn, clientIqn)
	result.userId, err = s.readAttribute(ctx, path, "auth", "userid")
	if err != nil {
		return nil, err
	}
	if result.userId == "" {
		return nil, nil
	}
	result.password, err = s.readAttribute(ctx, path, "auth", "password")
	if err != nil {
		return nil, err
	}
	if result.password == "" {
		return nil, nil
	}

	result.mutualUserId, err = s.readAttribute(ctx, path, "auth", "mutual_userid")
	if err != nil {
		return nil, err
	}
	if result.mutualUserId == "" {
		return nil, nil
	}
	result.mutualPassword, err = s.readAttribute(ctx, path, "auth", "mutual_password")
	if err != nil {
		return nil, err
	}
	if result.mutualPassword == "" {
		return nil, nil
	}

	return &result, nil
}

func (s targetcliSnippets) createClient(ctx context.Context, targetIqn string, clientIqn string) error {
	cmd := append(targetcliCmd, fmt.Sprintf("/iscsi/%s/tpg1/acls", targetIqn), "create", clientIqn)
	_, err := transport.RelaxedRun(ctx, s.tr, cmd, "This NodeACL already exists in configFS", transport.WithSudo(true))
	if err != nil {
		return fmt.Errorf("create client: %w", err)
	}
	return nil
}

func (s targetcliSnippets) setupClientAcl(ctx context.Context, targetIqn string, clientIqn string, acl chapAcl) error {
	path := fmt.Sprintf("/iscsi/%s/tpg1/acls/%s", targetIqn, clientIqn)

	attributes := map[string]string{
		"userid":          acl.userId,
		"password":        acl.password,
		"mutual_userid":   acl.mutualUserId,
		"mutual_password": acl.mutualPassword,
	}
	
	err := s.setAttributes(ctx, path, "attribute", attributes)
	if err != nil {
		return fmt.Errorf("configure attributes: %w", err)
	}

	return nil
}

func (s targetcliSnippets) deleteClientAcl(ctx context.Context, targetIqn string, clientIqn string) error {
	path := fmt.Sprintf("/iscsi/%s/tpg1/acls", targetIqn)
	cmd := append(targetcliCmd, path, "delete", clientIqn)
	_, err := transport.RelaxedRun(ctx, s.tr, cmd, "No such NodeACL in configfs", transport.WithSudo(true))
	if err != nil {
		return fmt.Errorf("delete client '%s' at '%s': %w", clientIqn, targetIqn, err)
	}
	return nil
}

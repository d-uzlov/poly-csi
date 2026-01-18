package shrtniscsi

import (
	"context"
	"encoding/json"
	"fmt"

	"meoe.io/poly-csi/pkg/transport"
)

type snippets struct {
	ctx       context.Context
	midcltCmd []string
}

func newSnippets(ctx context.Context, midcltCmd []string) snippets {
	return snippets{
		ctx:       ctx,
		midcltCmd: midcltCmd,
	}
}

type tnIscsiRequest struct {
	extentName string
	iqn        string
	zvol       string
}

func (s snippets) setupTarget(target tnIscsiRequest) error {
	tnExtents, err := s.getIscsiExtents(target.zvol)
	if err != nil {
		return err
	}
	var tnExtent tnExtentInfo

	switch len(tnExtents) {
	case 0:
		requestObject := fmt.Sprintf(`{"name":"%s","type":"DISK","disk":"zvol/%s"}`, target.extentName, target.zvol)
		cmd := append(s.midcltCmd, "call", "iscsi.extent.create", requestObject)
		_, err := transport.RelaxedRun(s.ctx, cmd, "Disk currently in use by extent "+target.extentName)
		if err != nil {
			return fmt.Errorf("%s: %w", cmd, err)
		}
		tnExtents, err = s.getIscsiExtents(target.zvol)
		if err != nil {
			return fmt.Errorf("getIscsiExtents: %w", err)
		}
		if tnExtents == nil {
			return fmt.Errorf("created extent but could not find it in the extent list")
		}
		tnExtent = tnExtents[0]
	case 1:
		tnExtent = tnExtents[0]
	default:
		return fmt.Errorf("found several extents associated with required zvol")
	}

	requestObject := fmt.Sprintf(`{"name":"%s"}`, target.iqn)
	cmd := append(s.midcltCmd, "call", "iscsi.target.create", requestObject)
	_, err = transport.RelaxedRun(s.ctx, cmd, "Target with this name already exists")
	if err != nil {
		return fmt.Errorf("%s: %w", cmd, err)
	}
	tnTarget, err := s.getIscsiTarget(target.iqn)
	if err != nil {
		return err
	}
	if tnTarget == nil {
		return fmt.Errorf("created iscsi target but could not find it in target list")
	}

	requestObject = fmt.Sprintf(`{"target":%d,"extent":%d}`, tnExtent.Id, tnTarget.Id)
	cmd = append(s.midcltCmd, "call", "iscsi.targetextent.create", requestObject)
	_, err = transport.RelaxedRun(s.ctx, cmd, "Extent is already in use")
	if err != nil {
		return fmt.Errorf("%s: %w", cmd, err)
	}

	return nil
}

type tnExtentInfo struct {
	Id   int    `json:"id,omitempty"`
	Name string `json:"name"`
	Disk string `json:"disk"`
}

func (s snippets) getIscsiExtents(zvol string) ([]tnExtentInfo, error) {
	requestObject := fmt.Sprintf(`[["disk","=","zvol/%s"]]`, zvol)
	cmd := append(s.midcltCmd, "call", "iscsi.extent.query", requestObject)
	stdout, err := transport.CheckedRunStdout(s.ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", cmd, err)
	}

	var result []tnExtentInfo
	err = json.Unmarshal([]byte(stdout), &result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

type tnTargetInfo struct {
	Id     int                 `json:"id,omitempty"`
	Name   string              `json:"name"`
	Groups []tnTargetGroupInfo `json:"groups"`
}

type tnTargetGroupInfo struct {
	Portal         int    `json:"portal"`
	InitiatorGroup *int   `json:"initiator"`
	AuthMethod     string `json:"authmethod"`
	Auth           *int   `json:"auth"`
}

func (s snippets) getIscsiTarget(name string) (*tnTargetInfo, error) {
	requestObject := fmt.Sprintf(`[["name","=","%s"]]`, name)
	cmd := append(s.midcltCmd, "call", "iscsi.target.query", requestObject)
	stdout, err := transport.CheckedRunStdout(s.ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", cmd, err)
	}

	var shareList []tnTargetInfo
	err = json.Unmarshal([]byte(stdout), &shareList)
	if err != nil {
		return nil, err
	}
	if len(shareList) > 1 {
		return nil, fmt.Errorf("found several extents for name %s", name)
	}
	if len(shareList) == 0 {
		return nil, nil
	}

	return &shareList[0], nil
}

type tnInitiatorGroupInfo struct {
	Id   int    `json:"id,omitempty"`
	Name string `json:"name"`
	Disk string `json:"disk"`
}

func (s snippets) getIscsiInitiatorGroups(iqn string) ([]tnInitiatorGroupInfo, error) {
	requestObject := fmt.Sprintf(`[["initiators","rin","%s"]]`, iqn)
	cmd := append(s.midcltCmd, "call", "iscsi.initiator.query", requestObject)
	stdout, err := transport.CheckedRunStdout(s.ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", cmd, err)
	}

	var result []tnInitiatorGroupInfo
	err = json.Unmarshal([]byte(stdout), &result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s snippets) detachGroup(targetIqn string, clientIqn string) error {
	tnTarget, err := s.getIscsiTarget(targetIqn)
	if err != nil {
		return err
	}
	if tnTarget == nil {
		return fmt.Errorf("iscsi target not found")
	}

	groups, err := s.getIscsiInitiatorGroups(clientIqn)
	var group tnInitiatorGroupInfo
	switch len(groups) {
	case 0:
		// is this OK if target group is already deleted?
		return nil
	case 1:
		group = groups[0]
	default:
		return fmt.Errorf("found several initiator groups for client iqn")
	}

	newGroups := []tnTargetGroupInfo{}
	for _, v := range tnTarget.Groups {
		if v.InitiatorGroup != nil && *v.InitiatorGroup != group.Id {
			newGroups = append(newGroups, v)
		}
	}
	tnTarget.Groups = newGroups

	id := tnTarget.Id
	tnTarget.Id = 0
	encoded, err := json.Marshal(tnTarget)
	if err != nil {
		return err
	}
	cmd := append(s.midcltCmd, "call", "iscsi.target.update", fmt.Sprintf("%d", id), string(encoded))
	err = transport.CheckedRun(s.ctx, cmd)
	if err != nil {
		return fmt.Errorf("%s: %w", cmd, err)
	}
	return nil
}

func (s snippets) attachGroup(targetIqn string, clientIqn string) error {
	tnTarget, err := s.getIscsiTarget(targetIqn)
	if err != nil {
		return err
	}
	if tnTarget == nil {
		return fmt.Errorf("iscsi target not found")
	}

	groups, err := s.getIscsiInitiatorGroups(clientIqn)
	var group tnInitiatorGroupInfo
	switch len(groups) {
	case 0:
		requestObject := fmt.Sprintf(`{"initiators":["%s"]}`, clientIqn)
		cmd := append(s.midcltCmd, "call", "iscsi.initiator.create", requestObject)
		err = transport.CheckedRun(s.ctx, cmd)
		if err != nil {
			return fmt.Errorf("%s: %w", cmd, err)
		}
		groups, err := s.getIscsiInitiatorGroups(clientIqn)
		if err != nil {
			return err
		}
		if len(groups) != 1 {
			return fmt.Errorf("created initiator group for %s and found %d matching groups", clientIqn, len(groups))
		}
		group = groups[0]
	case 1:
		group = groups[0]
	default:
		return fmt.Errorf("found several initiator groups for client iqn")
	}

	for _, v := range tnTarget.Groups {
		if v.InitiatorGroup != nil && *v.InitiatorGroup == group.Id {
			return nil
		}
	}
	tnTarget.Groups = append(tnTarget.Groups, tnTargetGroupInfo{
		Portal:         1,
		InitiatorGroup: &group.Id,
		AuthMethod:     "NONE",
	})

	id := tnTarget.Id
	tnTarget.Id = 0
	encoded, err := json.Marshal(tnTarget)
	if err != nil {
		return err
	}
	cmd := append(s.midcltCmd, "call", "iscsi.target.update", fmt.Sprintf("%d", id), string(encoded))
	err = transport.CheckedRun(s.ctx, cmd)
	if err != nil {
		return fmt.Errorf("%s: %w", cmd, err)
	}
	return nil
}

func (s snippets) removeIscsiTarget(iqn string) error {
	tnTarget, err := s.getIscsiTarget(iqn)
	if err != nil {
		return err
	}
	if tnTarget == nil {
		return nil
	}

	cmd := append(s.midcltCmd, "call", "iscsi.target.delete", fmt.Sprintf("%d", tnTarget.Id))
	err = transport.CheckedRun(s.ctx, cmd)
	if err != nil {
		return fmt.Errorf("%s: %w", cmd, err)
	}
	return nil
}

func (s snippets) removeIscsiExtents(zvol string) error {
	extents, err := s.getIscsiExtents(zvol)
	if err != nil {
		return err
	}
	for _, v := range extents {
		cmd := append(s.midcltCmd, "call", "iscsi.extent.delete", fmt.Sprintf("%d", v.Id))
		err = transport.CheckedRun(s.ctx, cmd)
		if err != nil {
			return fmt.Errorf("%s: %w", cmd, err)
		}
	}

	return nil
}

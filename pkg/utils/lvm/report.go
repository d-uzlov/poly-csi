package lvm

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"meoe.io/poly-csi/pkg/transport"
)

type ReportContainer struct {
	Report []Report `json:"report"`
}

type Report struct {
	Vg []VgReport `json:"vg"`
	Lv []LvReport `json:"lv"`
}

type VgReport struct {
	VgName    string `json:"vg_name"`
	PvCount   string `json:"pv_count"`
	LvCount   string `json:"lv_count"`
	SnapCount string `json:"snap_count"`
	VgAttr    string `json:"vg_attr"`
	VgSize    string `json:"vg_size"`
	VgFree    string `json:"vg_free"`
}

func (r *VgReport) GetFree() int64 {
	return parseNum(r.VgFree)
}

type LvReport struct {
	LvName string `json:"lv_name"`
	VgName string `json:"vg_name"`
	LvAttr string `json:"lv_attr"`
	LvSize string `json:"lv_size"`
	Origin string `json:"origin"`
}

func (r *LvReport) GetSize() int64 {
	return parseNum(r.LvSize)
}
func parseNum(value string) int64 {
	num, found := strings.CutSuffix(value, "B")
	if !found {
		return 0
	}
	result, err := strconv.ParseInt(num, 10, 64)
	if err != nil {
		return 0
	}
	return result
}

func (s *Snippets) ListVgs(ctx context.Context, name string) ([]VgReport, error) {
	cmd := append(lvmCmd, "vgs", "--reportformat=json", "--units=1b")
	if name != "" {
		cmd = append(cmd, name)
	}
	stdout, err := transport.CheckedRunStdout(ctx, s.tr, cmd, transport.WithSudo(true))
	if err != nil {
		return nil, fmt.Errorf("list vgs: %w", err)
	}
	var rc ReportContainer
	err = json.Unmarshal([]byte(stdout), &rc)
	if err != nil {
		return nil, fmt.Errorf("parse 'vgs' output as json: %w", err)
	}
	if len(rc.Report) != 1 {
		return nil, fmt.Errorf("parse 'vgs' output: expected single value but got %d", len(rc.Report))
	}
	return rc.Report[0].Vg, nil
}

func (s *Snippets) ListLvs(ctx context.Context, vgName string, lvName string) ([]LvReport, error) {
	cmd := append(lvmCmd, "lvs", "--reportformat=json", "--units=1b")
	if vgName != "" {
		if lvName != "" {
			cmd = append(cmd, s.MakeLvPath(vgName, lvName))
		} else {
			cmd = append(cmd, vgName)
		}
	}
	stdout, err := transport.CheckedRunStdout(ctx, s.tr, cmd, transport.WithSudo(true))
	if err != nil {
		return nil, fmt.Errorf("list lvs: %w", err)
	}
	var rc ReportContainer
	err = json.Unmarshal([]byte(stdout), &rc)
	if err != nil {
		return nil, fmt.Errorf("parse 'lvs' output as json: %w", err)
	}
	if len(rc.Report) != 1 {
		return nil, fmt.Errorf("parse 'lvs' output: expected single value but got %d", len(rc.Report))
	}
	return rc.Report[0].Lv, nil
}

package zfs

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"meoe.io/poly-csi/pkg/transport"
)

type zfsGetResult struct {
	Datasets map[string]DatasetState `json:"datasets"`
}

type DatasetState struct {
	Name       string            `json:"name"`
	Type       string            `json:"type"`
	Pool       string            `json:"pool"`
	Properties DatasetProperties `json:"properties"`
}

type DatasetProperties struct {
	Used           ZfsProperty `json:"used"`
	VolSize        ZfsProperty `json:"volsize"`
	LogicalUsed    ZfsProperty `json:"logicalused"`
	Referenced     ZfsProperty `json:"referenced"`
	Quota          ZfsProperty `json:"quota"`
	RefQuota       ZfsProperty `json:"refquota"`
	RefReservation ZfsProperty `json:"refreservation"`
	Available      ZfsProperty `json:"available"`
	MountPoint     ZfsProperty `json:"mountpoint"`
}

func (ds DatasetState) IsZvol() bool {
	return ds.Type != "FILESYSTEM"
}

func (ds DatasetState) GetSize() int64 {
	// TODO use better error handling?
	if ds.IsZvol() {
		i, err := ds.Properties.VolSize.GetInt()
		if err != nil {
			panic(err)
		}
		return i
	}
	if !ds.Properties.RefQuota.IsEmpty() {
		i, err := ds.Properties.RefQuota.GetInt()
		if err != nil {
			panic(err)
		}
		return i
	}
	i, err := ds.Properties.Quota.GetInt()
	if err != nil {
		panic(err)
	}
	return i
}

type ZfsProperty struct {
	Value  string `json:"value"`
	Source struct {
		Type string `json:"type"`
		Data string `json:"data"`
	} `json:"source"`
}

func (p ZfsProperty) GetInt() (int64, error) {
	if p.IsEmpty() {
		return 0, fmt.Errorf("value is empty")
	}
	i, err := strconv.ParseInt(p.Value, 10, 64)
	if err != nil {
		return 0, err
	}
	return i, nil
}

func (p ZfsProperty) IsEmpty() bool {
	return p.Value == "" || p.Value == "-" || p.Value == "none"
}

func (s *Snippets) GetProperties(ctx context.Context, path string) (*DatasetState, error) {
	properties := []string{
		"used",
		"volsize",
		"logicalused",
		"referenced",
		"quota",
		"reservation",
		"refquota",
		"refreservation",
		"available",
		"mountpoint",
	}
	// -p Display numbers in parsable (exact) values.
	cmd := append(s.zfsCmd, "get", strings.Join(properties, ","), path, "-p", "--json")
	runResult, err := s.tr.RunCommand(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("zfs get dataset '%s' properties: %w", path, err)
	}
	if runResult.ExitCode != 0 {
		if strings.Contains(string(runResult.Stderr), "dataset does not exist") {
			return nil, nil
		}
		err = transport.FormatRunError(runResult)
		return nil, fmt.Errorf("zfs get dataset '%s' properties: %w", path, err)
	}
	var info zfsGetResult
	err = json.Unmarshal(runResult.Stdout, &info)
	if err != nil {
		return nil, fmt.Errorf("parsing dataset info from %s: %w", path, err)
	}
	ds, ok := info.Datasets[path]
	if !ok {
		return nil, fmt.Errorf("command '%v' did not give results for dataset '%s'", cmd, path)
	}
	return &ds, nil
}

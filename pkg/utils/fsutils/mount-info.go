package fsutils

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

type MountInfo struct {
	Id         int
	ParentId   int
	MajorMinor string
	Root       string
	Source     string
	MountPoint string
	FsType     string
}

func (s *FsSnippets) FindMount(ctx context.Context, mountedPath string) (*MountInfo, error) {
	sr, err := s.tr.Cmds().Stat(ctx, mountedPath)
	if err != nil {
		return nil, fmt.Errorf("check mount path: %w", err)
	}
	if !sr.ParentExists || !sr.FileExists {
		return nil, nil
	}
	const mountinfoPath = "/proc/self/mountinfo"
	mountinfoContent, err := s.tr.Cmds().Read(ctx, mountinfoPath, false)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", mountinfoPath, err)
	}

	mis, err := parseMountinfoFile(string(mountinfoContent), sr.Path)
	if err != nil {
		return nil, err
	}
	for _, i := range mis {
		if i.MountPoint == sr.Path {
			return &i, nil
		}
	}
	return nil, nil
}

func parseMountinfoLine(line string) (MountInfo, error) {
	fields := strings.Fields(line)
	if len(fields) < 6 {
		return MountInfo{}, fmt.Errorf("less than 6 fields")
	}
	id, err := strconv.Atoi(fields[0])
	if err != nil {
		return MountInfo{}, fmt.Errorf("parse id: %w", err)
	}
	parentID, err := strconv.Atoi(fields[1])
	if err != nil {
		return MountInfo{}, fmt.Errorf("parse parent id: %w", err)
	}

	result := MountInfo{
		Id:         id,
		ParentId:   parentID,
		MajorMinor: fields[2],
		Root:       fields[3],
		MountPoint: fields[4],
	}
	i := 6
	for i < len(fields) {
		if fields[i] != "-" {
			break
		}
	}
	i++
	if len(fields)-i < 3 {
		return MountInfo{}, fmt.Errorf("less than 3 fields after '-'")
	}
	result.FsType = fields[i]
	result.Source = fields[i+1]

	return result, nil
}

func parseMountinfoFile(content string, mountPointFilter string) ([]MountInfo, error) {
	result := []MountInfo{}

	for line := range strings.SplitSeq(content, "\n") {
		if line == "" {
			continue
		}
		info, err := parseMountinfoLine(line)
		if err != nil {
			return nil, fmt.Errorf("parse '%v': %w", line, err)
		}

		if mountPointFilter != "" && info.MountPoint != mountPointFilter {
			continue
		}

		result = append(result, info)
	}

	return result, nil
}

package btrfs

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"meoe.io/poly-csi/pkg/transport"
)

func (s *Snippets) SetQuota(ctx context.Context, path string, quota int64) error {
	cmd := append(s.btrfsCmd, "qgroup", "limit", fmt.Sprintf("%d", quota), path)
	err := transport.CheckedRun(ctx, s.tr, cmd)
	if err != nil {
		return fmt.Errorf("%s: %w", cmd, err)
	}
	return nil
}

func (s *Snippets) GetQuotaInfo(ctx context.Context, path string) (*QuotaEntry, error) {
	cmd := append(s.btrfsCmd, "--format=json", "qgroup", "show", path, "-f")
	stdout, err := transport.CheckedRunStdout(ctx, s.tr, cmd)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", cmd, err)
	}

	var result BtrfsJsonResult
	err = json.Unmarshal([]byte(stdout), &result)
	if err != nil {
		return nil, fmt.Errorf("unmarshal 'qgroup show' as json: %w", err)
	}

	if result.Header.Version != "1" {
		return nil, fmt.Errorf("unknown format: header version is %s, expected '1'", result.Header.Version)
	}
	if len(result.QuotaEntries) != 1 {
		return nil, fmt.Errorf("expected single quota entry but specified subvolume has %d quota groups", len(result.QuotaEntries))
	}

	entry := result.QuotaEntries[0]
	return &entry, nil
}

type QuotaEntry struct {
	QGroupId      string   `json:"qgroupid"`
	Referenced    string   `json:"referenced"`
	MaxReferenced string   `json:"max_referenced"`
	Exclusive     string   `json:"exclusive"`
	MaxExclusive  string   `json:"max_exclusive"`
	Path          string   `json:"path"`
	Parents       []string `json:"parents"`
	Children      []string `json:"children"`
}

func (q *QuotaEntry) GetQuota() int64 {
	if q.MaxReferenced == "none" {
		return 0
	}
	size, err := strconv.ParseInt(q.MaxReferenced, 10, 64)
	if err != nil {
		return -1
	}
	return size
}

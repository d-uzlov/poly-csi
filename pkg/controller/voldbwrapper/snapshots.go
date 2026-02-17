package voldbwrapper

import (
	"context"
	"encoding/json"
	"fmt"
)

const snapshotsModule = "snapshots.json"

type snapshotsContextContainer struct {
	Entries map[string]string `json:"entries"`
}

type SnapshotInfo struct {
	SnapshotType   string `json:"snapshot_type"`
	StorageType    string `json:"storage_type"`
	SerializedData string `json:"serialized_data"`
}

func parseSnapshotInfo(content string) (*SnapshotInfo, error) {
	var result SnapshotInfo
	err := json.Unmarshal([]byte(content), &result)
	if err != nil {
		return nil, fmt.Errorf("parse json: %w", err)
	}
	return &result, nil
}

func (s *SnapshotInfo) serialize() (string, error) {
	encoded, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return "", fmt.Errorf("json marshal: %w", err)
	}
	return string(encoded), nil
}

func (w Wrapper) unsafeGetSnapshotsContainer(ctx context.Context) (snapshotsContextContainer, error) {
	var result snapshotsContextContainer
	content, err := w.entry.GetModule(ctx, snapshotsModule)
	if err != nil {
		return result, err
	}
	if content == nil {
		return result, nil
	}
	err = json.Unmarshal([]byte(content), &result)
	if err != nil {
		return result, fmt.Errorf("parse json: %w", err)
	}
	return result, nil
}

func (w Wrapper) SetSnapshotData(ctx context.Context, name string, snap *SnapshotInfo) error {
	release, err := w.entry.Lock(ctx)
	if err != nil {
		return err
	}
	defer release(ctx)

	pc, err := w.unsafeGetSnapshotsContainer(ctx)
	if err != nil {
		return err
	}

	if snap != nil {
		serialized, err := snap.serialize()
		if err != nil {
			return fmt.Errorf("serialize snapshot: %w", err)
		}
		if pc.Entries[name] == serialized {
			return nil
		}
		pc.Entries[name] = serialized
	} else {
		delete(pc.Entries, name)
	}

	encoded, err := json.MarshalIndent(pc, "", "  ")
	if err != nil {
		return fmt.Errorf("json marshal: %w", err)
	}
	return w.entry.SetModule(ctx, snapshotsModule, encoded)
}

func (w Wrapper) GetSnapshot(ctx context.Context, name string) (*SnapshotInfo, error) {
	release, err := w.entry.Lock(ctx)
	if err != nil {
		return nil, err
	}
	defer release(ctx)

	pc, err := w.unsafeGetSnapshotsContainer(ctx)
	if err != nil {
		return nil, err
	}

	serialized := pc.Entries[name]
	if serialized == "" {
		return nil, nil
	}

	snap, err := parseSnapshotInfo(serialized)
	if err != nil {
		return nil, err
	}

	return snap, nil
}

func (w Wrapper) GetFullSnapshotList(ctx context.Context) (map[string]SnapshotInfo, error) {
	release, err := w.entry.Lock(ctx)
	if err != nil {
		return nil, err
	}
	defer release(ctx)

	pc, err := w.unsafeGetSnapshotsContainer(ctx)
	if err != nil {
		return nil, err
	}

	result := map[string]SnapshotInfo{}
	for k, v := range pc.Entries {
		snap, err := parseSnapshotInfo(v)
		if err != nil || snap == nil {
			return nil, fmt.Errorf("failed to parse snapshot on key %v: %w", k, err)
		}
		result[k] = *snap
	}

	return result, nil
}

func (w Wrapper) DeleteSnapshotData(ctx context.Context, name string) error {
	return w.SetSnapshotData(ctx, name, nil)
}

func (w Wrapper) ClearSnapshotData(ctx context.Context) error {
	return w.entry.DeleteModule(ctx, snapshotsModule)
}

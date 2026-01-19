package voldbwrapper

import (
	"context"
	"encoding/json"
	"fmt"

	"meoe.io/poly-csi/pkg/controller/voldb"
)

type Wrapper struct {
	entry voldb.Entry
}

func New(entry voldb.Entry) Wrapper {
	return Wrapper{entry: entry}
}

type VolumeInfo struct {
	MounterId     string            `json:"mount_id"`
	TransportId   string            `json:"transport_id"`
	VolumeContext map[string]string `json:"volume_context"`
	AddressIds    []string          `json:"address_ids"`
	StorageType   string            `json:"storage_type"`
	StorageData   string            `json:"volume"`
	ShareType     string            `json:"share_type"`
	ShareData     string            `json:"share_data"`
}

type SnapshotInfo struct {
	SnapshotType   string `json:"snapshot_type"`
	StorageType    string `json:"storage_type"`
	SerializedData string `json:"serialized_data"`
}

type publishContextContainer struct {
	Entries map[string]string `json:"entries"`
}

const (
	mainVolumeModule = "volume.json"
	publishModule    = "publish.json"
)

func (w Wrapper) Delete(ctx context.Context) error {
	return w.entry.DeleteEntry(ctx)
}

func (w Wrapper) SetVolumeInfo(ctx context.Context, info VolumeInfo) error {
	encoded, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return fmt.Errorf("json marshal: %w", err)
	}
	return w.entry.SetModule(ctx, "volume.json", encoded)
}

func (w Wrapper) GetVolumeInfo(ctx context.Context) (*VolumeInfo, error) {
	content, err := w.entry.GetModule(ctx, "volume.json")
	if err != nil {
		return nil, err
	}
	if content == nil {
		return nil, nil
	}
	var result VolumeInfo
	err = json.Unmarshal(content, &result)
	if err != nil {
		return nil, fmt.Errorf("parse json: %w", err)
	}
	return &result, nil
}

func (w Wrapper) unsafeGetPublishContainer(ctx context.Context) (publishContextContainer, error) {
	var result publishContextContainer
	content, err := w.entry.GetModule(ctx, publishModule)
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

func (w Wrapper) SetPublishCtx(ctx context.Context, name string, data string) error {
	release, err := w.entry.Lock(ctx)
	if err != nil {
		return err
	}
	defer release(ctx)

	pc, err := w.unsafeGetPublishContainer(ctx)
	if err != nil {
		return err
	}

	if pc.Entries[name] == data {
		return nil
	}

	if data != "" {
		pc.Entries[name] = data
	} else {
		delete(pc.Entries, name)
	}

	encoded, err := json.MarshalIndent(pc, "", "  ")
	if err != nil {
		return fmt.Errorf("json marshal: %w", err)
	}
	return w.entry.SetModule(ctx, publishModule, encoded)
}

func (w Wrapper) GetPublishData(ctx context.Context, name string) (string, error) {
	release, err := w.entry.Lock(ctx)
	if err != nil {
		return "", err
	}
	defer release(ctx)

	pc, err := w.unsafeGetPublishContainer(ctx)
	if err != nil {
		return "", err
	}

	return pc.Entries[name], nil
}

func (w Wrapper) GetFullPublishData(ctx context.Context) (map[string]string, error) {
	release, err := w.entry.Lock(ctx)
	if err != nil {
		return nil, err
	}
	defer release(ctx)

	pc, err := w.unsafeGetPublishContainer(ctx)
	if err != nil {
		return nil, err
	}

	return pc.Entries, nil
}

func (w Wrapper) DeletePublishData(ctx context.Context, name string) error {
	return w.SetPublishCtx(ctx, name, "")
}

func (w Wrapper) ClearPublishData(ctx context.Context) error {
	return w.entry.DeleteModule(ctx, publishModule)
}

func (w Wrapper) CreateSnapshot(ctx context.Context, name string, data SnapshotInfo) error {
	encoded, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("json marshal: %w", err)
	}
	module := fmt.Sprintf("snapshot-%s.json", name)
	return w.entry.SetModule(ctx, module, encoded)
}

func (w Wrapper) GetSnapshot(ctx context.Context, name string) (*SnapshotInfo, error) {
	module := fmt.Sprintf("snapshot-%s.json", name)
	content, err := w.entry.GetModule(ctx, module)
	if err != nil {
		return nil, err
	}
	if content == nil {
		return nil, nil
	}
	var result SnapshotInfo
	err = json.Unmarshal(content, &result)
	if err != nil {
		return nil, fmt.Errorf("parse json: %w", err)
	}
	return &result, nil
}

func (w Wrapper) DeleteSnapshot(ctx context.Context, name string) error {
	module := fmt.Sprintf("snapshot-%s.json", name)
	err := w.entry.DeleteModule(ctx, module)
	if err != nil {
		return err
	}
	return nil
}

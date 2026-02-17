package voldbwrapper

import (
	"context"
	"encoding/json"
	"fmt"
)

const publishModule = "publish.json"

type publishContextContainer struct {
	Entries map[string]string `json:"entries"`
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

	if pc.Entries == nil {
		pc.Entries = map[string]string{}
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

	if pc.Entries == nil {
		return "", nil
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

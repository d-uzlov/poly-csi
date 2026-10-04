package shrvblade

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"meoe.io/poly-csi/pkg/controller/shares"
	"meoe.io/poly-csi/pkg/transport"
)

type share struct {
	tr transport.Transport `json:"-"`
	gp globalParams        `json:"-"`

	P Params `json:"params"`

	Name       string `json:"name"`
	Shelf      int    `json:"shelf"`
	Slot       int    `json:"slot"`
	DevicePath string `json:"devicePath"`
}

var _ shares.Share = &share{}

func parseShare(data string) (*share, error) {
	var result share
	err := json.Unmarshal([]byte(data), &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *share) Serialize() (string, error) {
	encoded, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func (s *share) Validate() error {
	if s.tr == nil {
		return fmt.Errorf("transport is nil")
	}
	if s.Name == "" {
		return fmt.Errorf("missing Name")
	}
	// if sh.DevicePath == "" {
	// 	return fmt.Errorf("missing DevicePath")
	// }
	return nil
}

func (s *share) PrepareShare(ctx context.Context) error {
	return nil
}

func (s *share) AttachPath(ctx context.Context, fsPath string, index int) error {
	snip := newSnippets(s.tr, s.gp)
	shelf, slot, err := snip.allocateSlot(ctx, s.Name)
	if err != nil {
		return err
	}
	s.Shelf = shelf
	s.Slot = slot

	s.DevicePath = fsPath

	err = snip.configureShare(ctx, s.Name, ShareConfig{
		Shelf: strconv.Itoa(s.Shelf),
		Slot:  strconv.Itoa(s.Slot),
		// TODO support more than one interface
		Interface: s.gp.Interfaces,
		Path:      s.DevicePath,
	})
	if err != nil {
		return fmt.Errorf("configureShare: %w", err)
	}

	return nil
}

func (s *share) AttachVolume(ctx context.Context, volume any, index int) error {
	return fmt.Errorf("unimplemented")
}

func (s *share) DeleteShare(ctx context.Context) error {
	snip := newSnippets(s.tr, s.gp)

	err := snip.removeShare(ctx, s.Name)
	if err != nil {
		return fmt.Errorf("remove share config: %w", err)
	}

	err = snip.freeSlot(ctx, s.Shelf, s.Slot, s.Name)
	if err != nil {
		return fmt.Errorf("remove share config: %w", err)
	}

	return nil
}

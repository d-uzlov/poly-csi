package shrvblade

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"text/template"
	"time"

	"meoe.io/poly-csi/pkg/transport"
)

var enableInstance = []string{"vblade_enable"}
var disableInstance = []string{"vblade_disable"}

var exportTemplate = template.Must(template.New("").Parse(`
SHELF={{ .Shelf }}
SLOT={{ .Slot }}
NETIF={{ .Interface }}
SOURCE={{ .Path }}
`))

type ShareConfig struct {
	Shelf     string
	Slot      string
	Interface string
	Path      string
}

type Snippets struct {
	tr transport.Transport
	gp globalParams

	minShelf int
	maxShelf int
}

type ShelfState struct {
	Shelf int
	Slots map[int]string
}

func newSnippets(tr transport.Transport, gp globalParams) Snippets {
	minShelf, err := strconv.Atoi(gp.MinShelf)
	if err != nil {
		// TODO add logs for failed validation
		minShelf = 10
	}
	maxShelf, err := strconv.Atoi(gp.MaxShelf)
	if err != nil {
		maxShelf = 20
	}

	return Snippets{
		tr:       tr,
		gp:       gp,
		minShelf: minShelf,
		maxShelf: maxShelf,
	}
}

const lockFile = "lock"

func (s *Snippets) readIndexUnsafe(ctx context.Context) (shelf int, slot int, err error) {
	path := fmt.Sprintf("%s/index", s.gp.ConfigDir)
	data, err := s.tr.Cmds().Read(ctx, path, false)
	if err != nil {
		return 0, 0, err
	}
	if data == nil {
		return s.minShelf, 0, nil
	}
	fields := strings.Fields(string(data))
	if len(fields) != 2 {
		return 0, 0, fmt.Errorf("expected 'shelf slot', got '%s'", string(data))
	}
	shelf, err = strconv.Atoi(fields[0])
	if err != nil {
		return 0, 0, err
	}
	slot, err = strconv.Atoi(fields[1])
	if err != nil {
		return 0, 0, err
	}
	return shelf, slot, nil
}

func (s *Snippets) writeIndexUnsafe(ctx context.Context, shelf int, slot int) error {
	path := fmt.Sprintf("%s/index", s.gp.ConfigDir)
	data := fmt.Sprintf("%d %d", shelf, slot)
	err := s.tr.Cmds().Write(ctx, path, []byte(data))
	if err != nil {
		return err
	}
	return nil
}

func (s *Snippets) readShelfStateUnsafe(ctx context.Context, shelf int) (ShelfState, error) {
	path := fmt.Sprintf("%s/shelf-%d.json", s.gp.ConfigDir, shelf)
	data, err := s.tr.Cmds().Read(ctx, path, false)
	if err != nil {
		return ShelfState{}, err
	}
	if data == nil {
		return ShelfState{
			Shelf: shelf,
			Slots: map[int]string{},
		}, nil
	}
	var result ShelfState
	err = json.Unmarshal(data, &result)
	if err != nil {
		return ShelfState{}, err
	}
	if result.Shelf != shelf {
		return ShelfState{}, fmt.Errorf("read shelf state at '%s': requested shelf %d, got data for %d", path, shelf, result.Shelf)
	}
	return result, nil
}

func (s *Snippets) writeShelfStateUnsafe(ctx context.Context, state ShelfState) error {
	path := fmt.Sprintf("%s/shelf-%d.json", s.gp.ConfigDir, state.Shelf)
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	err = s.tr.Cmds().Write(ctx, path, data)
	if err != nil {
		return err
	}
	return nil
}

func (s *Snippets) allocateSlot(ctx context.Context, name string) (shelf int, slot int, err error) {
	release, err := s.tr.Cmds().CreateLock(ctx, s.gp.ConfigDir+lockFile, 10*time.Second)
	if err != nil {
		return 0, 0, err
	}
	defer release(ctx)

	currentShelf, currentSlot, err := s.readIndexUnsafe(ctx)
	shelfState, err := s.readShelfStateUnsafe(ctx, currentShelf)
	if err != nil {
		return 0, 0, err
	}

	for {
		currentSlot = currentSlot + 1
		if currentSlot > 255 {
			currentSlot = 0
			currentShelf = currentShelf + 1
			if currentShelf > s.maxShelf {
				currentShelf = s.minShelf
				shelfState, err = s.readShelfStateUnsafe(ctx, currentShelf)
				if err != nil {
					return 0, 0, err
				}
			}
		}
		if shelfState.Slots[currentSlot] == "" {
			shelfState.Slots[currentSlot] = name
			err = s.writeShelfStateUnsafe(ctx, shelfState)
			if err != nil {
				return 0, 0, err
			}
			err = s.writeIndexUnsafe(ctx, currentShelf, currentSlot)
			if err != nil {
				return 0, 0, err
			}
			return currentShelf, currentSlot, nil
		}
	}
}

func (s *Snippets) freeSlot(ctx context.Context, shelf int, slot int, name string) error {
	release, err := s.tr.Cmds().CreateLock(ctx, s.gp.ConfigDir+lockFile, 10*time.Second)
	if err != nil {
		return err
	}
	defer release(ctx)

	shelfState, err := s.readShelfStateUnsafe(ctx, shelf)
	if shelfState.Slots[slot] == "" {
		return nil
	}
	if shelfState.Slots[slot] != name {
		return fmt.Errorf("current slot allocation does not match name")
	}
	shelfState.Slots[slot] = ""
	s.writeShelfStateUnsafe(ctx, shelfState)
	return nil
}

func (s *Snippets) configureShare(ctx context.Context, name string, share ShareConfig) error {
	file := fmt.Sprintf("%s/%s.conf", s.gp.ConfigDir, name)

	var buf bytes.Buffer
	err := exportTemplate.Execute(&buf, share)
	if err != nil {
		return err
	}

	err = s.tr.Cmds().Write(ctx, file, buf.Bytes())
	if err != nil {
		return fmt.Errorf("write '%s': %w", file, err)
	}
	cmd := append(enableInstance, name)
	err = transport.CheckedRun(ctx, s.tr, cmd)
	if err != nil {
		return fmt.Errorf("enable vblade service: %w", err)
	}
	return nil
}

func (s Snippets) removeShare(ctx context.Context, name string) error {
	file := fmt.Sprintf("%s/%s.conf", s.gp.ConfigDir, name)

	cmd := append(disableInstance, name)
	err := transport.CheckedRun(ctx, s.tr, cmd)
	if err != nil {
		return fmt.Errorf("disable vblade service: %w", err)
	}
	err = s.tr.Cmds().RmFile(ctx, file)
	if err != nil {
		return fmt.Errorf("remove '%s': %w", file, err)
	}
	return nil
}

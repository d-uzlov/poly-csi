package supervisor

import (
	"fmt"
	"sync"
)

type IdPool struct {
	mut      sync.Mutex
	inUse    Set[int]
	min, max int

	next     int
	capacity int
}

func NewIdPool(min, max int) *IdPool {
	return &IdPool{
		inUse:    Set[int]{},
		min:      min,
		max:      max,
		next:     min,
		capacity: max - min + 1,
	}
}

func (p *IdPool) Allocate() (int, error) {
	p.mut.Lock()
	defer p.mut.Unlock()

	if p.inUse.Len() >= p.capacity {
		return 0, fmt.Errorf("no free ids")
	}

	for {
		candidate := p.next

		p.next++
		if p.next > p.max {
			p.next = p.min
		}

		if !p.inUse.Has(candidate) {
			p.inUse.Add(candidate)
			return candidate, nil
		}
	}
}

func (p *IdPool) Grab(id int) error {
	p.mut.Lock()
	defer p.mut.Unlock()

	if id < p.min || id > p.max {
		return fmt.Errorf("id %d is out of range", id)
	}

	if p.inUse.Has(id) {
		return fmt.Errorf("id %d is already in use", id)
	}

	p.inUse.Add(id)
	return nil
}

func (p *IdPool) Release(id int) {
	p.mut.Lock()
	defer p.mut.Unlock()

	p.inUse.Remove(id)
}

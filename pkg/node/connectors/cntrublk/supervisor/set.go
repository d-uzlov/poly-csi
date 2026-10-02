package supervisor

type Set[T comparable] map[T]struct{}

func (s Set[T]) Add(val T) {
	s[val] = struct{}{}
}

func (s Set[T]) Has(val T) bool {
	_, exists := s[val]
	return exists
}

func (s Set[T]) Remove(val T) {
	delete(s, val)
}

func (s Set[T]) Len() int {
	return len(s)
}

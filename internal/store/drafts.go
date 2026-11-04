package store

func (s *Store) Load(id string) (Draft, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.load(id)
}

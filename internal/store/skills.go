package store

import (
	"slices"
)

func (s *Store) SkillsState() SkillState {
	s.mu.Lock()
	defer s.mu.Unlock()
	v := s.state.Skills
	v.Preferences.CustomLocations = slices.Clone(v.Preferences.CustomLocations)
	v.Bindings = make(map[string]SkillRecord, len(s.state.Skills.Bindings))
	for id, record := range s.state.Skills.Bindings {
		v.Bindings[id] = record
	}
	return v
}

func (s *Store) SaveSkills(v SkillState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	old := s.state.Skills
	s.state.Skills = v
	if err := s.save(); err != nil {
		s.state.Skills = old
		return err
	}
	return nil
}

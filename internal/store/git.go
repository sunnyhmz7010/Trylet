package store

import (
	"path/filepath"
	"strings"
)

func (s *Store) RecentRepositories() []GitRecent {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]GitRecent{}, s.state.GitRecent...)
}

func (s *Store) RememberGitRepository(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	old := s.state.GitRecent
	recent := []GitRecent{{Path: path, Name: filepath.Base(path)}}
	for _, item := range old {
		if !strings.EqualFold(item.Path, path) {
			recent = append(recent, item)
		}
	}
	if len(recent) > 12 {
		recent = recent[:12]
	}
	s.state.GitRecent = recent
	if err := s.save(); err != nil {
		s.state.GitRecent = old
		return err
	}
	return nil
}

func (s *Store) ForgetGitRepository(path string) ([]GitRecent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	old := s.state.GitRecent
	next := []GitRecent{}
	for _, item := range old {
		if !strings.EqualFold(item.Path, path) {
			next = append(next, item)
		}
	}
	s.state.GitRecent = next
	if err := s.save(); err != nil {
		s.state.GitRecent = old
		return nil, err
	}
	return append([]GitRecent{}, next...), nil
}

func (s *Store) GitPreferences() map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := map[string]string{}
	for key, value := range s.state.GitPreferences {
		result[key] = value
	}
	return result
}

func (s *Store) SaveGitPreference(key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state.GitPreferences == nil {
		s.state.GitPreferences = map[string]string{}
	}
	previous, existed := s.state.GitPreferences[key]
	s.state.GitPreferences[key] = value
	if err := s.save(); err != nil {
		if existed {
			s.state.GitPreferences[key] = previous
		} else {
			delete(s.state.GitPreferences, key)
		}
		return err
	}
	return nil
}

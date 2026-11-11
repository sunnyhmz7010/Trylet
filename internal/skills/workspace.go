package skills

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	storage "trylet/internal/store"
)

func skillPathID(path string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(filepath.Clean(path))))
	return hex.EncodeToString(sum[:])
}

func skillDirectory(path string) (string, error) {
	path = strings.TrimSpace(path)
	if !filepath.IsAbs(path) || strings.HasPrefix(path, `\\.\`) || strings.HasPrefix(path, `\\?\`) {
		return "", errors.New("请选择有效的绝对目录路径")
	}
	return filepath.Clean(path), nil
}

func (s *Service) skillWorkspace() (SkillWorkspace, error) {
	prefs := s.store.SkillsState().Preferences
	if prefs.Scope == "" {
		prefs.Scope = "user"
	}
	if prefs.CustomLocations == nil {
		prefs.CustomLocations = []storage.SkillCustomLocation{}
	}
	home := s.skillHome
	if home == "" {
		var err error
		home, err = os.UserHomeDir()
		if err != nil {
			return SkillWorkspace{}, fmt.Errorf("读取用户目录：%w", err)
		}
	}
	locations := []SkillLocation{}
	add := func(scope, tool, path string) {
		path = filepath.Clean(path)
		id := skillPathID(path)
		for _, location := range locations {
			if location.ID == id && location.Scope == scope {
				return
			}
		}
		info, err := os.Stat(path)
		locations = append(locations, SkillLocation{ID: id, Scope: scope, Tool: tool, Path: path, Exists: err == nil && info.IsDir(), ReadOnly: protectedSkillPath(path)})
	}
	for _, scope := range []string{"user", "project"} {
		base := home
		if scope == "project" {
			base = prefs.Project
			if base == "" {
				continue
			}
		}
		for _, tool := range []string{"agents", "codex", "claude"} {
			add(scope, tool, filepath.Join(base, "."+tool, "skills"))
		}
		for _, custom := range prefs.CustomLocations {
			if (scope == "user" && custom.Project == "") || (scope == "project" && strings.EqualFold(custom.Project, base)) {
				add(scope, "custom", custom.Path)
			}
		}
	}
	selected := false
	for _, location := range locations {
		if location.Scope == prefs.Scope && strings.EqualFold(location.Path, prefs.Location) {
			selected = true
			prefs.Location = location.Path
		}
	}
	if !selected {
		prefs.Location = ""
		for _, location := range locations {
			if location.Scope == prefs.Scope {
				prefs.Location = location.Path
				break
			}
		}
	}
	return SkillWorkspace{Preferences: prefs, Locations: locations}, nil
}

func (s *Service) GetSkillWorkspace() (SkillWorkspace, error) {
	s.skillsMu.Lock()
	defer s.skillsMu.Unlock()
	return s.skillWorkspace()
}

func (s *Service) SaveSkillPreferences(prefs storage.SkillPreferences) (SkillWorkspace, error) {
	s.skillsMu.Lock()
	defer s.skillsMu.Unlock()
	if prefs.Scope != "user" && prefs.Scope != "project" {
		return SkillWorkspace{}, errors.New("Skills 作用域无效")
	}
	if prefs.Project != "" {
		var err error
		prefs.Project, err = skillDirectory(prefs.Project)
		if err != nil {
			return SkillWorkspace{}, err
		}
	}
	for i := range prefs.CustomLocations {
		var err error
		custom := &prefs.CustomLocations[i]
		custom.Path, err = skillDirectory(custom.Path)
		if err != nil {
			return SkillWorkspace{}, err
		}
		if custom.Project != "" {
			custom.Project, err = skillDirectory(custom.Project)
			if err != nil {
				return SkillWorkspace{}, err
			}
		}
	}
	state := s.store.SkillsState()
	state.Preferences = prefs
	if err := s.store.SaveSkills(state); err != nil {
		return SkillWorkspace{}, err
	}
	return s.skillWorkspace()
}

func (s *Service) skillLocation(id string) (SkillLocation, error) {
	workspace, err := s.skillWorkspace()
	if err != nil {
		return SkillLocation{}, err
	}
	for _, location := range workspace.Locations {
		if location.ID == id {
			return location, nil
		}
	}
	return SkillLocation{}, errors.New("Skills 位置已改变，请刷新后重试")
}

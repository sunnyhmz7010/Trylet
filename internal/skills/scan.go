package skills

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	gitrepo "trylet/internal/git"
	storage "trylet/internal/store"
)

func skillEntries(root *os.Root, rootName string) ([]SkillEntry, error) {
	entries := []SkillEntry{}
	err := fs.WalkDir(root.FS(), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == ".git" || d.Name() == "node_modules" || strings.HasPrefix(d.Name(), ".Trylet-skill-")) {
			return fs.SkipDir
		}
		if !d.Type().IsRegular() || !strings.EqualFold(d.Name(), "SKILL.md") {
			return nil
		}
		dir := filepath.ToSlash(filepath.Dir(path))
		name := filepath.Base(dir)
		if dir == "." {
			name = rootName
		}
		entries = append(entries, SkillEntry{Name: name, Path: dir})
		return nil
	})
	return entries, err
}

func (s *Service) ScanSkills(locationID string) ([]SkillInstallation, error) {
	s.skillsMu.Lock()
	defer s.skillsMu.Unlock()
	location, err := s.skillLocation(locationID)
	if err != nil {
		return nil, err
	}
	state := s.store.SkillsState()
	items := []SkillInstallation{}
	seen := map[string]bool{}
	bindingsChanged := false
	root, err := os.OpenRoot(location.Path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("读取 Skills 目录：%w", err)
	}
	if root != nil {
		defer root.Close()
		children, err := fs.ReadDir(root.FS(), ".")
		if err != nil {
			return nil, err
		}
		for _, child := range children {
			if strings.HasPrefix(child.Name(), ".Trylet-skill-") || child.Name() == ".git" {
				continue
			}
			info, err := child.Info()
			if err != nil {
				return nil, fmt.Errorf("读取 %s：%w", child.Name(), err)
			}
			linked := skillIsLink(info)
			if !info.IsDir() && !linked {
				continue
			}
			path := filepath.Join(location.Path, child.Name())
			id := skillPathID(path)
			record, bound := state.Bindings[id]
			item := SkillInstallation{ID: id, LocationID: location.ID, Name: child.Name(), Path: path, Skills: []SkillEntry{}, Linked: linked, ReadOnly: protectedSkillPath(path)}
			if !linked {
				sub, readErr := root.OpenRoot(child.Name())
				if readErr == nil {
					item.Skills, readErr = skillEntries(sub, child.Name())
					sub.Close()
				}
				if readErr != nil {
					item.Issue = "读取技能失败：" + readErr.Error()
				}
			}
			if len(item.Skills) == 0 && !linked && !bound && item.Issue == "" {
				continue
			}
			if !linked && !item.ReadOnly && len(item.Skills) > 0 {
				detected, detectErr := skillGitRecord(path)
				if detectErr == nil {
					if !bound || record.Mode != detected.Mode || record.Repository != detected.Repository || record.Revision != detected.Revision {
						state.Bindings[id] = detected
						bindingsChanged = true
					}
					record, bound = detected, true
				} else if !errors.Is(detectErr, os.ErrNotExist) {
					item.Issue = "读取 Git 来源失败：" + detectErr.Error()
					bound = false
				}
			}
			if bound {
				item.Repository, item.SourcePath, item.Mode, item.Revision = record.Repository, record.SourcePath, record.Mode, record.Revision
			}
			seen[id] = true
			items = append(items, item)
		}
	}
	for id, record := range state.Bindings {
		if seen[id] || !strings.EqualFold(record.Root, location.Path) {
			continue
		}
		items = append(items, SkillInstallation{ID: id, LocationID: location.ID, Name: record.Name, Path: filepath.Join(record.Root, record.Name), Skills: []SkillEntry{}, Repository: record.Repository, SourcePath: record.SourcePath, Mode: record.Mode, Revision: record.Revision, Missing: true, Issue: "安装目录已不存在，可更新恢复或删除绑定"})
		if record.Mode == "git" {
			items[len(items)-1].Issue = "Git 克隆目录已不存在，请重新克隆或删除绑定"
		}
	}
	if bindingsChanged {
		if err := s.store.SaveSkills(state); err != nil {
			return nil, fmt.Errorf("保存 Git 来源失败：%w", err)
		}
	}
	sort.Slice(items, func(i, j int) bool { return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name) })
	return items, nil
}

func skillGitRecord(path string) (storage.SkillRecord, error) {
	if _, err := os.Lstat(filepath.Join(path, ".git")); err != nil {
		return storage.SkillRecord{}, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return storage.SkillRecord{}, err
	}
	if !info.IsDir() || skillIsLink(info) || protectedSkillPath(path) {
		return storage.SkillRecord{}, errors.New("不能更新链接或系统内置 Git 目录")
	}
	remote, revision, err := gitrepo.Source(path)
	if err != nil {
		return storage.SkillRecord{}, err
	}
	if strings.HasPrefix(remote, "git@github.com:") {
		remote = strings.TrimPrefix(remote, "git@github.com:")
	} else if strings.HasPrefix(remote, "ssh://git@github.com/") {
		remote = strings.TrimPrefix(remote, "ssh://git@github.com/")
	} else if !strings.Contains(remote, "://") {
		return storage.SkillRecord{}, errors.New("origin 不是支持的 GitHub HTTPS 或 SSH 仓库地址")
	}
	repository, err := skillRepository(remote)
	if err != nil {
		return storage.SkillRecord{}, errors.New("origin 不是支持的 GitHub HTTPS 或 SSH 仓库地址")
	}
	return storage.SkillRecord{ID: skillPathID(path), Root: filepath.Dir(path), Name: filepath.Base(path), Repository: repository, SourcePath: ".", Mode: "git", Revision: revision}, nil
}

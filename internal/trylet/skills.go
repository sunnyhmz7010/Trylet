package trylet

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"syscall"
	"time"
)

func (s *Store) skillsState() skillState {
	s.mu.Lock()
	defer s.mu.Unlock()
	v := s.state.Skills
	v.Preferences.CustomLocations = slices.Clone(v.Preferences.CustomLocations)
	v.Bindings = make(map[string]skillRecord, len(s.state.Skills.Bindings))
	for id, record := range s.state.Skills.Bindings {
		v.Bindings[id] = record
	}
	return v
}

func (s *Store) saveSkills(v skillState) error {
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
	prefs := s.store.skillsState().Preferences
	if prefs.Scope == "" {
		prefs.Scope = "user"
	}
	if prefs.CustomLocations == nil {
		prefs.CustomLocations = []SkillCustomLocation{}
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

func (s *Service) SaveSkillPreferences(prefs SkillPreferences) (SkillWorkspace, error) {
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
	state := s.store.skillsState()
	state.Preferences = prefs
	if err := s.store.saveSkills(state); err != nil {
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
	state := s.store.skillsState()
	items := []SkillInstallation{}
	seen := map[string]bool{}
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
	}
	sort.Slice(items, func(i, j int) bool { return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name) })
	return items, nil
}

func validSkillName(name string) bool {
	if name == "" || name == "." || name == ".." || strings.TrimRight(name, ". ") != name || strings.ContainsAny(name, `<>:"/\|?*`) {
		return false
	}
	for _, r := range name {
		if r < 32 {
			return false
		}
	}
	return filepath.IsLocal(name)
}

func protectedSkillPath(path string) bool {
	paths := []string{path}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		paths = append(paths, resolved)
	}
	for _, candidate := range paths {
		for _, component := range strings.Split(filepath.ToSlash(candidate), "/") {
			if strings.EqualFold(component, ".system") {
				return true
			}
		}
	}
	return false
}

func validSkillTarget(name string) bool {
	return validSkillName(name) && !strings.EqualFold(name, ".system") && !strings.EqualFold(name, ".git") && !strings.HasPrefix(strings.ToLower(name), ".trylet-skill-")
}

// Go 将 Windows junction 标记为 ModeIrregular，需同时识别原生 reparse 属性。
func skillIsLink(info fs.FileInfo) bool {
	data, ok := info.Sys().(*syscall.Win32FileAttributeData)
	return info.Mode()&os.ModeSymlink != 0 || (ok && data.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0)
}

func skillManifest(root *os.Root) (map[string]string, error) {
	files := map[string]string{}
	err := fs.WalkDir(root.FS(), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || path == "." {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if skillIsLink(info) {
			target, err := root.Readlink(filepath.FromSlash(path))
			files[path] = "link:" + target
			return err
		}
		if d.IsDir() {
			files[path+"/"] = "directory"
			return nil
		}
		if !d.Type().IsRegular() {
			return fmt.Errorf("不支持的文件类型：%s", path)
		}
		f, err := root.Open(filepath.FromSlash(path))
		if err != nil {
			return err
		}
		hash := sha256.New()
		_, readErr := io.Copy(hash, f)
		err = errors.Join(readErr, f.Close())
		files[path] = hex.EncodeToString(hash.Sum(nil))
		return err
	})
	return files, err
}

func skillDigest(files map[string]string) string {
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	hash := sha256.New()
	for _, path := range paths {
		fmt.Fprintf(hash, "%d:%s:%s\n", len(path), path, files[path])
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func changedSkillFiles(before, after map[string]string) []string {
	changed := []string{}
	for path, hash := range before {
		if after[path] != hash {
			changed = append(changed, path)
		}
	}
	for path := range after {
		if _, exists := before[path]; !exists {
			changed = append(changed, path)
		}
	}
	sort.Strings(changed)
	return changed
}

func copySkillTree(source, target *os.Root) error {
	return fs.WalkDir(source.FS(), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || path == "." {
			return err
		}
		path = filepath.FromSlash(path)
		if d.IsDir() {
			return target.MkdirAll(path, 0755)
		}
		if !d.Type().IsRegular() {
			return fmt.Errorf("技能中含有链接或特殊文件：%s", path)
		}
		in, err := source.Open(path)
		if err != nil {
			return err
		}
		out, err := target.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
		if err != nil {
			return errors.Join(err, in.Close())
		}
		_, copyErr := io.Copy(out, in)
		return errors.Join(copyErr, out.Sync(), out.Close(), in.Close())
	})
}

func currentSkillFiles(root *os.Root, name string) (map[string]string, error) {
	info, err := root.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || skillIsLink(info) {
		return nil, errors.New("安装目录已变成文件或链接，已保留原内容")
	}
	sub, err := root.OpenRoot(name)
	if err != nil {
		return nil, err
	}
	defer sub.Close()
	return skillManifest(sub)
}

// 暂存和备份均放在目标根目录内，保证跨磁盘安装时仍可用重命名提交和回滚。
func (s *Service) replaceSkill(source *os.Root, record skillRecord, update bool, baseline string) (string, error) {
	if _, err := skillDirectory(record.Root); err != nil {
		return "", err
	}
	if !validSkillTarget(record.Name) || protectedSkillPath(filepath.Join(record.Root, record.Name)) {
		return "", errors.New("技能目录名无效或属于受保护目录")
	}
	if err := os.MkdirAll(record.Root, 0755); err != nil {
		return "", err
	}
	root, err := os.OpenRoot(record.Root)
	if err != nil {
		return "", err
	}
	defer root.Close()
	stage := ".Trylet-skill-stage-" + newID()
	backup := ".Trylet-skill-backup-" + newID()
	if err := root.Mkdir(stage, 0700); err != nil {
		return "", err
	}
	defer func() {
		if err := root.RemoveAll(stage); err != nil {
			log.Printf("清理技能暂存目录 %s：%v", filepath.Join(record.Root, stage), err)
		}
	}()
	staged, err := root.OpenRoot(stage)
	if err != nil {
		return "", err
	}
	err = copySkillTree(source, staged)
	if err == nil {
		record.Files, err = skillManifest(staged)
	}
	err = errors.Join(err, staged.Close())
	if err != nil {
		return "", err
	}
	info, statErr := root.Lstat(record.Name)
	exists := statErr == nil
	if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return "", statErr
	}
	if !update && exists {
		return "", errors.New("同名目录已存在，已保留；如已绑定此仓库，请使用更新")
	}
	if update {
		files, err := currentSkillFiles(root, record.Name)
		if err != nil {
			return "", err
		}
		if skillDigest(files) != baseline {
			return "", errors.New("本地文件在下载期间发生变化，已保留，请重新点击更新")
		}
	}
	if exists {
		if !info.IsDir() || skillIsLink(info) {
			return "", errors.New("目标不是普通目录，已保留原内容")
		}
		if err := root.Rename(record.Name, backup); err != nil {
			return "", err
		}
	}
	if err := root.Rename(stage, record.Name); err != nil {
		if exists {
			err = errors.Join(err, root.Rename(backup, record.Name))
		}
		return "", err
	}
	state := s.store.skillsState()
	state.Bindings[record.ID] = record
	if err := s.store.saveSkills(state); err != nil {
		rollbackErr := root.Rename(record.Name, stage)
		if exists && rollbackErr == nil {
			rollbackErr = root.Rename(backup, record.Name)
		}
		return "", errors.Join(fmt.Errorf("保存仓库绑定失败：%w", err), rollbackErr)
	}
	if exists {
		if err := root.RemoveAll(backup); err != nil {
			return fmt.Sprintf("已更新，旧备份清理失败，保留于 %s：%v", filepath.Join(record.Root, backup), err), nil
		}
	}
	return "", nil
}

func (s *Service) InstallSkills(previewID, locationID string, paths []string) ([]SkillInstallResult, error) {
	s.skillsMu.Lock()
	defer s.skillsMu.Unlock()
	download, err := s.skillDownload(previewID)
	if err != nil {
		return nil, err
	}
	location, err := s.skillLocation(locationID)
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, errors.New("请先勾选要安装的技能")
	}
	for _, path := range paths {
		if _, found := download.candidate(path); !found {
			return nil, errors.New("所选技能不属于本次下载，请重新识别")
		}
	}
	results := []SkillInstallResult{}
	seen := map[string]bool{}
	for _, path := range paths {
		if seen[path] {
			continue
		}
		seen[path] = true
		candidate, found := download.candidate(path)
		if !found {
			return nil, errors.New("所选技能不属于本次下载，请重新识别")
		}
		result := SkillInstallResult{SourcePath: path, Name: candidate.Name, Path: filepath.Join(location.Path, candidate.Name)}
		source, openErr := os.OpenRoot(filepath.Join(download.directory, "repository", filepath.FromSlash(path)))
		if openErr != nil {
			result.Error = openErr.Error()
		} else {
			record := skillRecord{ID: skillPathID(result.Path), Root: location.Path, Name: candidate.Name, Repository: download.Repository, SourcePath: path, Mode: download.Mode, Revision: download.Revision}
			result.Warning, openErr = s.replaceSkill(source, record, false, "")
			source.Close()
			if openErr != nil {
				result.Error = openErr.Error()
			}
		}
		results = append(results, result)
	}
	return results, nil
}

func (s *Service) UpdateSkill(id, confirmation string) (SkillUpdateResult, error) {
	s.skillsMu.Lock()
	defer s.skillsMu.Unlock()
	record, exists := s.store.skillsState().Bindings[id]
	if !exists || !validSkillTarget(record.Name) || skillPathID(filepath.Join(record.Root, record.Name)) != id {
		return SkillUpdateResult{}, errors.New("技能尚未绑定仓库，或安装记录无效")
	}
	root, err := os.OpenRoot(record.Root)
	files := map[string]string{}
	present := false
	if err == nil {
		_, statErr := root.Lstat(record.Name)
		present = statErr == nil
		files, err = currentSkillFiles(root, record.Name)
		root.Close()
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return SkillUpdateResult{}, err
	}
	baseline := skillDigest(files)
	changes := changedSkillFiles(record.Files, files)
	// 确认只适用于本次看到的文件快照；确认期间再次编辑必须重新确认。
	token := skillPathID(id + record.Revision + baseline)
	if present && len(changes) > 0 && confirmation != token {
		return SkillUpdateResult{NeedsConfirmation: true, Confirmation: token, Changes: changes}, nil
	}
	download, err := s.prepareSkillDownload(record.Repository, record.Mode == "extracted")
	if err != nil {
		return SkillUpdateResult{}, err
	}
	defer s.removeSkillDownload(download.ID)
	if record.Mode == "extracted" {
		if _, found := download.candidate(record.SourcePath); !found {
			return SkillUpdateResult{}, fmt.Errorf("仓库中的 %s 已不存在，已保留本地技能", record.SourcePath)
		}
	}
	source, err := os.OpenRoot(filepath.Join(download.directory, "repository", filepath.FromSlash(record.SourcePath)))
	if err != nil {
		return SkillUpdateResult{}, fmt.Errorf("读取更新内容失败，已保留本地技能：%w", err)
	}
	defer source.Close()
	if record.Revision == download.Revision && len(changes) == 0 {
		return SkillUpdateResult{Message: "已是最新版本"}, nil
	}
	record.Revision = download.Revision
	warning, err := s.replaceSkill(source, record, true, baseline)
	if err != nil {
		return SkillUpdateResult{}, err
	}
	return SkillUpdateResult{Message: "已更新到仓库最新版本", Warning: warning}, nil
}

func (s *Service) DeleteSkill(locationID, name string) (string, error) {
	s.skillsMu.Lock()
	defer s.skillsMu.Unlock()
	if !validSkillTarget(name) {
		return "", errors.New("不能删除系统目录或无效路径")
	}
	location, err := s.skillLocation(locationID)
	if err != nil {
		return "", err
	}
	if protectedSkillPath(filepath.Join(location.Path, name)) {
		return "", errors.New("不能删除系统内置技能")
	}
	id := skillPathID(filepath.Join(location.Path, name))
	state := s.store.skillsState()
	originalRecord, bound := state.Bindings[id]
	root, err := os.OpenRoot(location.Path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	exists := false
	linked := false
	backup := ".Trylet-skill-delete-" + newID()
	if root != nil {
		defer root.Close()
		info, err := root.Lstat(name)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		exists = err == nil
		if exists {
			linked = skillIsLink(info)
			if !linked {
				if !info.IsDir() {
					return "", errors.New("目标已变成文件，取消删除")
				}
				if !bound {
					sub, err := root.OpenRoot(name)
					if err != nil {
						return "", err
					}
					entries, err := skillEntries(sub, name)
					sub.Close()
					if err != nil || len(entries) == 0 {
						return "", errors.Join(errors.New("目标已不包含技能，取消删除"), err)
					}
				}
			}
			if err := root.Rename(name, backup); err != nil {
				return "", fmt.Errorf("无法移除 %s，已保留原目录：%w", filepath.Join(location.Path, name), err)
			}
		}
	}
	if !exists && !bound {
		return "", errors.New("技能目录已不存在，请刷新列表")
	}
	delete(state.Bindings, id)
	if err := s.store.saveSkills(state); err != nil {
		if exists {
			err = errors.Join(err, root.Rename(backup, name))
		}
		return "", err
	}
	if exists {
		var err error
		if linked {
			err = root.Remove(backup)
		} else {
			err = root.RemoveAll(backup)
		}
		if err != nil {
			// 删除可能只完成了一部分，保留原路径和来源，供重试或更新恢复。
			restorePathErr := root.Rename(backup, name)
			var restoreBindingErr error
			if bound {
				restored := s.store.skillsState()
				restored.Bindings[id] = originalRecord
				restoreBindingErr = s.store.saveSkills(restored)
			}
			message := "删除未完成，剩余文件已回到原目录：" + filepath.Join(location.Path, name) + "；请解除占用后在文件管理器中检查清理"
			if bound && restoreBindingErr == nil {
				message = "删除未完成，剩余文件和仓库绑定已保留；可重试删除，或更新恢复缺失文件"
			}
			if restorePathErr != nil {
				message = "删除未完成，剩余文件保留于 " + filepath.Join(location.Path, backup)
			}
			return "", errors.Join(fmt.Errorf("%s：%w", message, err), restorePathErr, restoreBindingErr)
		}
	}
	return "", nil
}

type skillDownload struct {
	SkillPreview
	directory string
	created   time.Time
}

func (d *skillDownload) candidate(path string) (SkillEntry, bool) {
	for _, candidate := range d.Candidates {
		if candidate.Path == path {
			return candidate, true
		}
	}
	return SkillEntry{}, false
}

func (s *Service) skillDownload(id string) (*skillDownload, error) {
	download := s.skillDownloads[id]
	if download == nil || time.Since(download.created) > 30*time.Minute {
		return nil, errors.New("下载预览已过期，请重新识别仓库")
	}
	return download, nil
}

func (s *Service) removeSkillDownload(id string) {
	if download := s.skillDownloads[id]; download != nil {
		if err := os.RemoveAll(download.directory); err != nil {
			log.Printf("清理技能下载 %s：%v", download.directory, err)
		}
		delete(s.skillDownloads, id)
	}
}

func (s *Service) DiscardSkillPreview(id string) {
	s.skillsMu.Lock()
	defer s.skillsMu.Unlock()
	s.removeSkillDownload(id)
}

package skills

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"trylet/internal/identity"
	storage "trylet/internal/store"
)

// 暂存和备份均放在目标根目录内，保证跨磁盘安装时仍可用重命名提交和回滚。
func (s *Service) replaceSkill(source *os.Root, record storage.SkillRecord, update bool, baseline string) (string, error) {
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
	stage := ".Trylet-skill-stage-" + identity.New()
	backup := ".Trylet-skill-backup-" + identity.New()
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
		if _, err := root.Lstat(filepath.Join(record.Name, ".git")); err == nil {
			return "", errors.New("安装目录已变为 Git 工作区，请刷新后使用 Git 更新")
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
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
	state := s.store.SkillsState()
	state.Bindings[record.ID] = record
	if err := s.store.SaveSkills(state); err != nil {
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
			record := storage.SkillRecord{ID: skillPathID(result.Path), Root: location.Path, Name: candidate.Name, Repository: download.Repository, SourcePath: path, Mode: download.Mode, Revision: download.Revision}
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

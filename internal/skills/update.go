package skills

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	gitrepo "trylet/internal/git"
	"trylet/internal/identity"
	storage "trylet/internal/store"
)

func (s *Service) updateGitSkill(record storage.SkillRecord) (SkillUpdateResult, error) {
	path := filepath.Join(record.Root, record.Name)
	revision, err := gitrepo.FastForward(path)
	if err != nil {
		return SkillUpdateResult{}, err
	}
	result := SkillUpdateResult{Message: "已更新当前 Git 分支"}
	if revision == record.Revision {
		result.Message = "已是最新版本（当前 Git 分支无需快进）"
	}
	record.Revision = revision
	state := s.store.SkillsState()
	state.Bindings[record.ID] = record
	if err := s.store.SaveSkills(state); err != nil {
		result.Warning = "Git 更新已完成，但来源记录保存失败：" + err.Error()
	}
	return result, nil
}

func (s *Service) UpdateSkill(id, confirmation string) (SkillUpdateResult, error) {
	s.skillsMu.Lock()
	defer s.skillsMu.Unlock()
	record, exists := s.store.SkillsState().Bindings[id]
	if !exists || !validSkillTarget(record.Name) || skillPathID(filepath.Join(record.Root, record.Name)) != id {
		return SkillUpdateResult{}, errors.New("技能尚未绑定仓库，或安装记录无效")
	}
	// 即使安装记录仍是 ZIP 模式，也不能用快照替换后来放入的 Git 工作区。
	detected, detectErr := skillGitRecord(filepath.Join(record.Root, record.Name))
	if record.Mode == "git" || !errors.Is(detectErr, os.ErrNotExist) {
		if detectErr != nil {
			return SkillUpdateResult{}, fmt.Errorf("读取 Git 来源失败，请刷新列表或检查克隆目录：%w", detectErr)
		}
		return s.updateGitSkill(detected)
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
	downloadID := identity.New()
	s.skillsMu.Unlock()
	_, err = s.PrepareSkillInstall(context.Background(), downloadID, record.Repository, record.Mode == "extracted")
	s.skillsMu.Lock()
	if err != nil {
		return SkillUpdateResult{}, err
	}
	defer s.removeSkillDownload(downloadID)
	download, err := s.skillDownload(downloadID)
	if err != nil {
		return SkillUpdateResult{}, err
	}
	if current, exists := s.store.SkillsState().Bindings[id]; !exists || !reflect.DeepEqual(current, record) {
		return SkillUpdateResult{}, errors.New("安装记录在下载期间发生变化，请刷新后重新更新")
	}
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

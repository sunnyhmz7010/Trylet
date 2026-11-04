package skills

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"trylet/internal/identity"
)

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
	state := s.store.SkillsState()
	originalRecord, bound := state.Bindings[id]
	root, err := os.OpenRoot(location.Path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	exists := false
	linked := false
	backup := ".Trylet-skill-delete-" + identity.New()
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
	if err := s.store.SaveSkills(state); err != nil {
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
				restored := s.store.SkillsState()
				restored.Bindings[id] = originalRecord
				restoreBindingErr = s.store.SaveSkills(restored)
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

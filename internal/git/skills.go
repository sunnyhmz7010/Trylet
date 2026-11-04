package git

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

func Source(path string) (string, string, error) {
	reader := NewGitReader(nil, "", nil)
	origin, err := reader.text(path, "remote", "get-url", "origin")
	if err != nil {
		return "", "", err
	}
	revision, err := reader.text(path, "rev-parse", "--verify", "HEAD")
	return origin, revision, err
}

// FastForward 只更新干净且跟踪 origin 的分支，不覆盖、暂存或重置用户改动。
func FastForward(path string) (string, error) {
	reader := NewGitReader(nil, "", nil)
	if err := reader.checkFilters(path); err != nil {
		return "", err
	}
	status, err := reader.text(path, "status", "--porcelain=v1", "--untracked-files=normal", "--ignore-submodules=none")
	if err != nil {
		return "", err
	}
	if status != "" {
		return "", errors.New("Git 工作区存在本地改动，请先自行提交或处理后再更新；不会覆盖或自动暂存")
	}
	upstream, err := reader.text(path, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}")
	if err != nil || !strings.HasPrefix(upstream, "origin/") {
		return "", errors.New("当前分支未跟踪 origin 的上游分支，请先在 Git 中设置后再更新")
	}
	reader.timeout = 2 * time.Minute
	if _, err := reader.text(path, "-c", "core.hooksPath="+os.DevNull, "fetch", "--no-recurse-submodules", "origin"); err != nil {
		return "", fmt.Errorf("Git 获取更新失败：%w", err)
	}
	if _, err := reader.text(path, "-c", "core.hooksPath="+os.DevNull, "merge", "--ff-only", "--no-edit", "--no-autostash", "--no-overwrite-ignore", upstream); err != nil {
		return "", fmt.Errorf("Git 快进更新失败，未执行覆盖或重置：%w", err)
	}
	revision, err := reader.text(path, "rev-parse", "--verify", "HEAD")
	if err != nil {
		return "", fmt.Errorf("Git 更新已完成，但读取当前版本失败：%w", err)
	}
	return revision, nil
}

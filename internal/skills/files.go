package skills

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
)

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

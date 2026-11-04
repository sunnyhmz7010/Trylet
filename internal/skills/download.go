package skills

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	gitrepo "trylet/internal/git"
	githubapi "trylet/internal/github"
	"trylet/internal/identity"
)

const skillArchiveLimit = 100 << 20

const skillExtractLimit = 512 << 20

func skillRepository(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if strings.Contains(raw, "://") {
		u, err := url.Parse(raw)
		if err != nil || u.Scheme != "https" || !strings.EqualFold(u.Host, "github.com") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return "", errors.New("请输入 GitHub HTTPS 仓库地址或 owner/repo")
		}
		raw = strings.Trim(u.Path, "/")
	}
	raw = strings.TrimSuffix(strings.TrimSuffix(raw, "/"), ".git")
	parts := strings.Split(raw, "/")
	_, repositoryErr := githubapi.RepositoryEndpoint(raw)
	if len(parts) != 2 || repositoryErr != nil || parts[0] == "." || parts[0] == ".." || !validSkillTarget(parts[1]) {
		return "", errors.New("仓库地址格式无效，请使用 https://github.com/owner/repo 或 owner/repo")
	}
	return raw, nil
}

func (s *Service) skillJSON(ctx context.Context, endpoint string, out any) error {
	resp, err := s.publisher.Download(ctx, endpoint)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(out); err != nil {
		return fmt.Errorf("读取 GitHub 返回数据：%w", err)
	}
	return nil
}

func (s *Service) prepareSkillDownload(ctx context.Context, repository string, extract bool, progress func(string)) (*skillDownload, error) {
	repository, err := skillRepository(repository)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	if progress != nil {
		progress("connecting")
	}
	var info struct {
		DefaultBranch string `json:"default_branch"`
	}
	base := "/repos/" + repository
	if err := s.skillJSON(ctx, base, &info); err != nil {
		return nil, err
	}
	if info.DefaultBranch == "" {
		return nil, errors.New("仓库未提供默认分支")
	}
	var commit struct{ SHA string }
	if err := s.skillJSON(ctx, base+"/commits/"+url.PathEscape(info.DefaultBranch), &commit); err != nil {
		return nil, err
	}
	if !gitrepo.ValidCommit(commit.SHA) {
		return nil, errors.New("仓库返回的提交版本无效")
	}
	dir, err := os.MkdirTemp(s.store.TempDir(), "skills-")
	if err != nil {
		return nil, err
	}
	success := false
	defer func() {
		if !success {
			cleanupSkillDownload(dir)
		}
	}()
	if progress != nil {
		progress("downloading")
	}
	resp, err := s.publisher.Download(ctx, base+"/zipball/"+commit.SHA)
	if err != nil {
		return nil, err
	}
	archive := filepath.Join(dir, "archive.zip")
	f, err := os.OpenFile(archive, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		resp.Body.Close()
		return nil, err
	}
	size, downloadErr := io.Copy(f, io.LimitReader(resp.Body, skillArchiveLimit+1))
	err = errors.Join(downloadErr, f.Close(), resp.Body.Close())
	if err != nil {
		return nil, fmt.Errorf("下载仓库失败：%w", err)
	}
	if size > skillArchiveLimit {
		return nil, errors.New("仓库压缩包超过 100 MiB，请使用较小的技能仓库")
	}
	repositoryDir := filepath.Join(dir, "repository")
	if progress != nil {
		progress("extracting")
	}
	if err := extractSkillArchive(ctx, archive, repositoryDir); err != nil {
		return nil, err
	}
	if progress != nil {
		progress("scanning")
	}
	root, err := os.OpenRoot(repositoryDir)
	if err != nil {
		return nil, err
	}
	_, name, _ := strings.Cut(repository, "/")
	entries, err := skillEntries(root, name)
	root.Close()
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, errors.New("仓库中未发现 SKILL.md，请检查仓库地址")
	}
	mode := "extracted"
	if !extract {
		mode = "repository"
		entries = []SkillEntry{{Name: name, Path: "."}}
	}
	download := &skillDownload{SkillPreview: SkillPreview{ID: identity.New(), Repository: repository, Revision: commit.SHA, Mode: mode, Candidates: entries}, directory: dir, created: time.Now()}
	success = true
	return download, nil
}

func cleanupSkillDownload(dir string) {
	if err := os.RemoveAll(dir); err != nil {
		log.Printf("清理技能下载 %s：%v", dir, err)
	}
}

func (s *Service) PrepareSkillInstall(ctx context.Context, id, repository string, extract bool) (SkillPreview, error) {
	if !identity.Valid(id) {
		return SkillPreview{}, errors.New("下载标识无效")
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	s.skillsMu.Lock()
	for key, download := range s.skillDownloads {
		if time.Since(download.created) > 30*time.Minute {
			s.removeSkillDownload(key)
		}
	}
	if len(s.skillDownloads) >= 4 || s.skillDownloads[id] != nil {
		s.skillsMu.Unlock()
		return SkillPreview{}, errors.New("尚有未关闭的下载预览，请先关闭再试")
	}
	if s.skillDownloads == nil {
		s.skillDownloads = map[string]*skillDownload{}
	}
	pending := &skillDownload{SkillPreview: SkillPreview{ID: id}, created: time.Now(), cancel: cancel}
	s.skillDownloads[id] = pending
	s.skillsMu.Unlock()
	defer func() {
		s.skillsMu.Lock()
		defer s.skillsMu.Unlock()
		if s.skillDownloads[id] == pending {
			delete(s.skillDownloads, id)
		}
	}()
	// 网络与解压不持有 Skills 锁，扫描和取消不必等待下载结束。
	download, err := s.prepareSkillDownload(ctx, repository, extract, func(phase string) {
		if s.skillProgress != nil {
			s.skillProgress(SkillDownloadProgress{ID: id, Phase: phase})
		}
	})
	if err != nil {
		return SkillPreview{}, err
	}
	s.skillsMu.Lock()
	defer s.skillsMu.Unlock()
	if ctx.Err() != nil || s.skillDownloads[id] != pending {
		cleanupSkillDownload(download.directory)
		return SkillPreview{}, context.Canceled
	}
	download.ID = id
	s.skillDownloads[id] = download
	return download.SkillPreview, nil
}

func extractSkillArchive(ctx context.Context, archive, directory string) error {
	z, err := zip.OpenReader(archive)
	if err != nil {
		return fmt.Errorf("仓库压缩包无法读取：%w", err)
	}
	defer z.Close()
	if len(z.File) > 20000 {
		return errors.New("仓库文件超过 20000 项，无法作为轻量技能仓库导入")
	}
	type archivePath struct {
		name string
		dir  bool
	}
	seen := map[string]archivePath{}
	prefix := ""
	var total uint64
	for _, file := range z.File {
		if err := ctx.Err(); err != nil {
			return err
		}
		name := strings.TrimSuffix(file.Name, "/")
		if name == "" || strings.Contains(name, "\\") || path.Clean(name) != name || strings.HasPrefix(name, "/") {
			return fmt.Errorf("压缩包含有越界或无效路径：%s", file.Name)
		}
		parts := strings.Split(name, "/")
		for i, component := range parts {
			if !validSkillName(component) {
				return fmt.Errorf("压缩包含有 Windows 不支持的路径：%s", file.Name)
			}
			part := strings.Join(parts[:i+1], "/")
			key := strings.ToLower(part)
			dir := i < len(parts)-1 || file.FileInfo().IsDir()
			if previous, found := seen[key]; found && (previous.name != part || previous.dir != dir || !dir) {
				return fmt.Errorf("压缩包存在同名或大小写冲突路径：%s", file.Name)
			}
			seen[key] = archivePath{name: part, dir: dir}
		}
		if prefix == "" {
			prefix = parts[0]
		}
		if prefix != parts[0] || (len(parts) == 1 && !file.FileInfo().IsDir()) {
			return errors.New("仓库压缩包的根目录结构无效")
		}
		if !file.Mode().IsRegular() && !file.Mode().IsDir() {
			return fmt.Errorf("压缩包含有链接或特殊文件：%s", file.Name)
		}
		if file.UncompressedSize64 > skillExtractLimit-total {
			return errors.New("仓库解压后超过 512 MiB，已取消导入")
		}
		total += file.UncompressedSize64
	}
	if err := os.Mkdir(directory, 0700); err != nil {
		return err
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return err
	}
	defer root.Close()
	for _, file := range z.File {
		if err := ctx.Err(); err != nil {
			return err
		}
		_, relative, _ := strings.Cut(strings.TrimSuffix(file.Name, "/"), "/")
		if relative == "" {
			continue
		}
		name := filepath.FromSlash(relative)
		if file.FileInfo().IsDir() {
			if err := root.MkdirAll(name, 0755); err != nil {
				return err
			}
			continue
		}
		if err := root.MkdirAll(filepath.Dir(name), 0755); err != nil {
			return err
		}
		in, err := file.Open()
		if err != nil {
			return err
		}
		out, err := root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if err != nil {
			return errors.Join(err, in.Close())
		}
		written, copyErr := io.Copy(out, io.LimitReader(in, int64(file.UncompressedSize64)+1))
		if written > int64(file.UncompressedSize64) {
			copyErr = errors.New("解压内容超过声明的大小")
		}
		if err := errors.Join(copyErr, out.Close(), in.Close()); err != nil {
			return fmt.Errorf("解压 %s 失败：%w", relative, err)
		}
	}
	return nil
}

type skillDownload struct {
	SkillPreview
	directory string
	created   time.Time
	cancel    context.CancelFunc
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
	if download == nil || download.directory == "" || time.Since(download.created) > 30*time.Minute {
		return nil, errors.New("下载预览已过期，请重新识别仓库")
	}
	return download, nil
}

func (s *Service) removeSkillDownload(id string) {
	if download := s.skillDownloads[id]; download != nil {
		if download.cancel != nil {
			download.cancel()
		}
		if download.directory != "" {
			cleanupSkillDownload(download.directory)
		}
		delete(s.skillDownloads, id)
	}
}

func (s *Service) DiscardSkillPreview(id string) {
	s.skillsMu.Lock()
	defer s.skillsMu.Unlock()
	s.removeSkillDownload(id)
}

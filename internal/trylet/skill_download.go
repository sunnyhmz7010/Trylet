package trylet

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
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
	if len(parts) != 2 || !repoPattern.MatchString(raw) || parts[0] == "." || parts[0] == ".." || !validSkillTarget(parts[1]) {
		return "", errors.New("仓库地址格式无效，请使用 https://github.com/owner/repo 或 owner/repo")
	}
	return raw, nil
}

// 公开资源先匿名访问，鉴权或限流时再使用已有令牌；跨域下载不转发令牌。
func (s *Service) skillGET(ctx context.Context, endpoint string) (*http.Response, error) {
	client := *s.publisher.client
	client.Timeout = 2 * time.Minute
	origin, err := url.Parse(s.publisher.base)
	if err != nil {
		return nil, err
	}
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) > 5 {
			return errors.New("GitHub 下载重定向次数过多")
		}
		sameOrigin := req.URL.Scheme == origin.Scheme && req.URL.Host == origin.Host
		if !sameOrigin {
			req.Header.Del("Authorization")
			if req.URL.Scheme != "https" || req.URL.Host != "codeload.github.com" {
				return errors.New("GitHub 下载重定向到不受支持的地址")
			}
		}
		return nil
	}
	token := ""
	for attempt := 0; attempt < 2; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.publisher.base+endpoint, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("User-Agent", "Trylet/0.1")
		req.Header.Set("X-GitHub-Api-Version", "2026-03-10")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("连接 GitHub 失败：%w", err)
		}
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return resp, nil
		}
		data, readErr := io.ReadAll(io.LimitReader(resp.Body, 16<<10))
		resp.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		if attempt == 0 && (resp.StatusCode == 401 || resp.StatusCode == 403 || resp.StatusCode == 404 || resp.StatusCode == 429) {
			token, err = s.publisher.token()
			if err != nil {
				return nil, fmt.Errorf("读取 GitHub 令牌失败：%w", err)
			}
			if token != "" {
				continue
			}
		}
		var diagnostic struct{ Message string }
		_ = json.Unmarshal(data, &diagnostic)
		if resp.StatusCode == 404 || resp.StatusCode == 401 {
			diagnostic.Message = "仓库不可访问或为空；私有仓库请在设置中保存有 Contents 读取权限的令牌"
		} else if diagnostic.Message == "" {
			diagnostic.Message = http.StatusText(resp.StatusCode)
		}
		return nil, fmt.Errorf("GitHub 请求失败（%d）：%s", resp.StatusCode, diagnostic.Message)
	}
	return nil, errors.New("GitHub 鉴权失败")
}

func (s *Service) skillJSON(ctx context.Context, endpoint string, out any) error {
	resp, err := s.skillGET(ctx, endpoint)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(out); err != nil {
		return fmt.Errorf("读取 GitHub 返回数据：%w", err)
	}
	return nil
}

func (s *Service) prepareSkillDownload(repository string, extract bool) (*skillDownload, error) {
	repository, err := skillRepository(repository)
	if err != nil {
		return nil, err
	}
	for id, download := range s.skillDownloads {
		if time.Since(download.created) > 30*time.Minute {
			s.removeSkillDownload(id)
		}
	}
	if len(s.skillDownloads) >= 4 {
		return nil, errors.New("尚有未关闭的下载预览，请先关闭再试")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
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
	if !commitPattern.MatchString(commit.SHA) {
		return nil, errors.New("仓库返回的提交版本无效")
	}
	dir, err := os.MkdirTemp(s.store.tempDir, "skills-")
	if err != nil {
		return nil, err
	}
	success := false
	defer func() {
		if !success {
			cleanupSkillDownload(dir)
		}
	}()
	resp, err := s.skillGET(ctx, base+"/zipball/"+commit.SHA)
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
	if err := extractSkillArchive(archive, repositoryDir); err != nil {
		return nil, err
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
	if len(entries) == 0 {
		return nil, errors.New("仓库中未发现 SKILL.md，请检查仓库地址")
	}
	mode := "extracted"
	if !extract {
		mode = "repository"
		entries = []SkillEntry{{Name: name, Path: "."}}
	}
	download := &skillDownload{SkillPreview: SkillPreview{ID: newID(), Repository: repository, Revision: commit.SHA, Mode: mode, Candidates: entries}, directory: dir, created: time.Now()}
	if s.skillDownloads == nil {
		s.skillDownloads = map[string]*skillDownload{}
	}
	s.skillDownloads[download.ID] = download
	success = true
	return download, nil
}

func cleanupSkillDownload(dir string) {
	if err := os.RemoveAll(dir); err != nil {
		log.Printf("清理技能下载 %s：%v", dir, err)
	}
}

func (s *Service) PrepareSkillInstall(repository string, extract bool) (SkillPreview, error) {
	s.skillsMu.Lock()
	defer s.skillsMu.Unlock()
	download, err := s.prepareSkillDownload(repository, extract)
	if err != nil {
		return SkillPreview{}, err
	}
	return download.SkillPreview, nil
}

func extractSkillArchive(archive, directory string) error {
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

package trylet

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

type Publisher struct {
	base   string
	client *http.Client
	token  func() (string, error)
}

func NewPublisher(token func() (string, error)) *Publisher {
	return &Publisher{base: "https://api.github.com", client: &http.Client{Timeout: 20 * time.Second}, token: token}
}

type apiError struct {
	status  int
	message string
}

func (e *apiError) Error() string {
	return fmt.Sprintf("GitHub 请求失败（%d）：%s", e.status, e.message)
}

func (p *Publisher) request(ctx context.Context, method, endpoint string, body any, out any) error {
	var input io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		input = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, p.base+endpoint, input)
	if err != nil {
		return err
	}
	token, err := p.token()
	if err != nil {
		return err
	}
	if token == "" {
		return errors.New("请先在设置中保存 GitHub 仓库访问令牌")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "Trylet/0.1")
	req.Header.Set("X-GitHub-Api-Version", "2026-03-10")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("连接 GitHub 失败：%w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var e struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(data, &e)
		if e.Message == "" {
			e.Message = http.StatusText(resp.StatusCode)
		}
		if resp.StatusCode == 404 {
			e.Message = "仓库、分支或文件不可访问，请检查目标及令牌权限"
		}
		return &apiError{resp.StatusCode, e.Message}
	}
	if out == nil {
		return nil
	}
	if err = json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("GitHub 返回的数据无法读取：%w", err)
	}
	return nil
}

var repoPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)

func repositoryEndpoint(repository string) (string, error) {
	if !repoPattern.MatchString(repository) {
		return "", errors.New("仓库请填写 owner/repo，例如 sunny/practise-daily")
	}
	return "/repos/" + repository, nil
}

func escapedPath(v string) string {
	parts := strings.Split(v, "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	return strings.Join(parts, "/")
}

func validRemotePath(v string, allowEmpty bool) bool {
	if v == "" {
		return allowEmpty
	}
	if strings.HasPrefix(v, "/") || strings.ContainsAny(v, "\\\x00\r\n:") {
		return false
	}
	for _, s := range strings.Split(v, "/") {
		if s == "" || s == "." || s == ".." {
			return false
		}
	}
	return true
}

func (p *Publisher) Repository(ctx context.Context, repository string) (RepositoryInfo, error) {
	base, err := repositoryEndpoint(repository)
	if err != nil {
		return RepositoryInfo{}, err
	}
	var info struct {
		DefaultBranch string `json:"default_branch"`
		Size          int    `json:"size"`
	}
	if err = p.request(ctx, "GET", base, nil, &info); err != nil {
		return RepositoryInfo{}, err
	}
	result := RepositoryInfo{DefaultBranch: info.DefaultBranch, Branches: []string{}}
	for page := 1; ; page++ {
		var batch []struct {
			Name string `json:"name"`
		}
		err = p.request(ctx, "GET", fmt.Sprintf("%s/branches?per_page=100&page=%d", base, page), nil, &batch)
		if err != nil {
			return result, err
		}
		for _, b := range batch {
			result.Branches = append(result.Branches, b.Name)
		}
		if len(batch) < 100 {
			break
		}
	}
	if len(result.Branches) == 0 && info.DefaultBranch != "" {
		result.Branches = []string{info.DefaultBranch}
	}
	return result, nil
}

type treeEntry struct {
	Path string `json:"path"`
	Type string `json:"type"`
	SHA  string `json:"sha"`
}
type gitTree struct {
	Tree      []treeEntry `json:"tree"`
	Truncated bool        `json:"truncated"`
}

func (p *Publisher) Directory(ctx context.Context, target PublishTarget, dir string) (Directory, error) {
	result := Directory{Path: dir, Names: []string{}, Exists: true}
	base, err := repositoryEndpoint(target.Repository)
	if err != nil {
		return result, err
	}
	if !validRemotePath(dir, true) {
		return result, errors.New("目录路径无效")
	}
	if target.Branch == "" {
		return result, errors.New("请选择分支")
	}
	ref := target.Branch
	segments := []string{}
	if dir != "" {
		segments = strings.Split(dir, "/")
	}
	for n := 0; ; n++ {
		var tree gitTree
		err = p.request(ctx, "GET", base+"/git/trees/"+url.PathEscape(ref), nil, &tree)
		var ae *apiError
		if errors.As(err, &ae) && ae.status == 409 {
			result.Exists = false
			return result, nil
		}
		if err != nil {
			return result, err
		}
		if tree.Truncated {
			return result, errors.New("该目录过大，GitHub 未返回完整内容，请选择更小的目录")
		}
		if n == len(segments) {
			for _, entry := range tree.Tree {
				if entry.Type == "tree" {
					result.Names = append(result.Names, entry.Path)
				}
			}
			sort.Strings(result.Names)
			return result, nil
		}
		found := false
		for _, entry := range tree.Tree {
			if entry.Type == "tree" && entry.Path == segments[n] {
				ref = entry.SHA
				found = true
				break
			}
		}
		if !found {
			result.Exists = false
			return result, nil
		}
	}
}

type remoteFile struct {
	SHA      string `json:"sha"`
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
	HTMLURL  string `json:"html_url"`
	Type     string `json:"type"`
}

func (p *Publisher) remote(ctx context.Context, t PublishTarget) (*remoteFile, string, error) {
	base, err := repositoryEndpoint(t.Repository)
	if err != nil {
		return nil, "", err
	}
	var file remoteFile
	err = p.request(ctx, "GET", base+"/contents/"+escapedPath(t.Path)+"?ref="+url.QueryEscape(t.Branch), nil, &file)
	var ae *apiError
	if errors.As(err, &ae) && ae.status == 404 {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", err
	}
	if file.Type != "file" || file.Encoding != "base64" {
		return nil, "", errors.New("目标不是可读取的文本文件，请选择其他路径")
	}
	b, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(file.Content, "\n", ""))
	if err != nil {
		return nil, "", err
	}
	if !utf8.Valid(b) {
		return nil, "", errors.New("远端文件不是 UTF-8 文本，请另存到新路径")
	}
	return &file, string(b), nil
}

func (p *Publisher) Publish(ctx context.Context, r PublishRequest) (PublishResult, error) {
	if !validID(r.DraftID) || len(r.Code) > 1024*1024 {
		return PublishResult{}, errors.New("发布草稿无效或超过 1 MiB")
	}
	if !validRemotePath(r.Target.Path, false) || extension(r.Language) == "" || strings.ToLower(path.Ext(r.Target.Path)) != extension(r.Language) {
		return PublishResult{}, errors.New("文件路径无效，或扩展名与当前语言不匹配")
	}
	if strings.TrimSpace(r.Message) == "" || r.Target.Branch == "" {
		return PublishResult{}, errors.New("请填写提交说明并选择分支")
	}
	base, err := repositoryEndpoint(r.Target.Repository)
	if err != nil {
		return PublishResult{}, err
	}
	file, content, err := p.remote(ctx, r.Target)
	if err != nil {
		return PublishResult{}, err
	}
	if file != nil && content == r.Code {
		return PublishResult{Status: "synced", Message: "内容已同步", Record: &PublishRecord{Target: r.Target, SHA: file.SHA, FileURL: file.HTMLURL}}, nil
	}
	conflict := func(file *remoteFile, content, message string) PublishResult {
		sha := ""
		if file != nil {
			sha = file.SHA
		}
		return PublishResult{Status: "conflict", RemoteCode: content, SHA: sha, Message: message}
	}
	if file != nil && (!r.Confirmed || r.ExpectedSHA != file.SHA) {
		return conflict(file, content, "目标已存在不同内容，请选择如何处理"), nil
	}
	if file == nil && r.Confirmed && r.ExpectedSHA != "" {
		return conflict(nil, "", "远端文件已被删除，请重新确认创建"), nil
	}
	payload := map[string]any{"message": r.Message, "content": base64.StdEncoding.EncodeToString([]byte(r.Code)), "branch": r.Target.Branch}
	if file != nil {
		payload["sha"] = file.SHA
	}
	var response struct {
		Content remoteFile `json:"content"`
		Commit  struct {
			HTMLURL string `json:"html_url"`
		} `json:"commit"`
	}
	err = p.request(ctx, "PUT", base+"/contents/"+escapedPath(r.Target.Path), payload, &response)
	if err != nil {
		// 请求可能已提交但响应丢失；重读确认后才报告同步成功。
		latest, latestCode, checkErr := p.remote(ctx, r.Target)
		if checkErr == nil && latest != nil && latestCode == r.Code {
			return PublishResult{Status: "synced", Message: "已核对远端，内容已同步", Record: &PublishRecord{Target: r.Target, SHA: latest.SHA, FileURL: latest.HTMLURL}}, nil
		}
		var ae *apiError
		if checkErr == nil && errors.As(err, &ae) && (ae.status == 409 || ae.status == 422) && latest != nil {
			return conflict(latest, latestCode, "远端内容刚发生变化，请重新选择"), nil
		}
		return PublishResult{}, err
	}
	return PublishResult{Status: "published", Message: "已提交到 GitHub", Record: &PublishRecord{Target: r.Target, SHA: response.Content.SHA, FileURL: response.Content.HTMLURL, CommitURL: response.Commit.HTMLURL}}, nil
}

package trylet

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/windows"
)

const gitReadLimit = 16 * 1024 * 1024
const gitCommitFormat = "%H%x00%P%x00%an%x00%ae%x00%cn%x00%ce%x00%aI%x00%s%x00"

type gitReadError struct {
	Kind    string `json:"kind"`
	Message string `json:"message"`
}

func (e *gitReadError) Error() string     { b, _ := json.Marshal(e); return string(b) }
func gitError(kind, message string) error { return &gitReadError{kind, message} }

type gitRepo struct{ root, gitDir, commonDir string }
type gitCapture struct {
	bytes.Buffer
	limit     int
	truncated bool
}

func (c *gitCapture) Write(b []byte) (int, error) {
	n := len(b)
	if len(b) > c.limit-c.Len() {
		b = b[:max(0, c.limit-c.Len())]
		c.truncated = true
	}
	_, _ = c.Buffer.Write(b)
	return n, nil
}

type GitReader struct {
	store      *Store
	tempRoot   string
	binary     string
	timeout    time.Duration
	lfsCommand string
	gate       chan struct{}
	mu         sync.Mutex
	repos      map[string]gitRepo
	active     string
	activeTab  bool
	visible    bool
	stopWatch  context.CancelFunc
	emit       func(string)
}

func NewGitReader(store *Store, tempRoot string, emit func(string)) *GitReader {
	binary, _ := exec.LookPath("git.exe")
	if binary == "" {
		for _, root := range []string{os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)"), filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs")} {
			candidate := filepath.Join(root, "Git", "cmd", "git.exe")
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				binary = candidate
				break
			}
		}
	}
	executable, _ := os.Executable()
	quoted := "'" + strings.ReplaceAll(filepath.ToSlash(executable), "'", "'\\''") + "'"
	return &GitReader{store: store, tempRoot: tempRoot, binary: binary, lfsCommand: quoted + " --trylet-lfs-clean", gate: make(chan struct{}, 3), repos: map[string]gitRepo{}}
}
func (g *GitReader) run(root string, args []string, input []byte, limit int, allowed ...int) ([]byte, bool, error) {
	if g.binary == "" {
		return nil, false, gitError("gitUnavailable", "没有检测到 Git，请指定或安装本机 Git 后重试。")
	}
	timeout := g.timeout
	if timeout == 0 {
		timeout = 8 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	select {
	case g.gate <- struct{}{}:
		defer func() { <-g.gate }()
	case <-ctx.Done():
		return nil, false, ctx.Err()
	}
	prefix := []string{"--no-pager", "--no-optional-locks", "-c", "core.fsmonitor=false", "-c", "log.showSignature=false", "-c", "diff.autoRefreshIndex=false", "-c", "filter.lfs.process=", "-c", "filter.lfs.required=true", "-c", "filter.lfs.clean=" + g.lfsCommand, "-c", "core.quotepath=false"}
	if root != "" {
		prefix = append(prefix, "-C", root)
	}
	cmd := hiddenCommand(ctx, g.binary, append(prefix, args...)...)
	cmd.WaitDelay = time.Second
	env := []string{}
	for _, v := range os.Environ() {
		key, _, _ := strings.Cut(v, "=")
		if !strings.HasPrefix(strings.ToUpper(key), "GIT_") {
			env = append(env, v)
		}
	}
	cmd.Env = append(env, "GIT_OPTIONAL_LOCKS=0", "GIT_NO_LAZY_FETCH=1", "GIT_TERMINAL_PROMPT=0", "LC_ALL=C")
	if input != nil {
		cmd.Stdin = bytes.NewReader(input)
	}
	out, diagnostic := &gitCapture{limit: limit}, &gitCapture{limit: 32000}
	cmd.Stdout, cmd.Stderr = out, diagnostic
	job, err := createJob()
	if err != nil {
		return nil, false, err
	}
	defer windows.CloseHandle(job)
	if err = cmd.Start(); err != nil {
		return nil, false, gitError("gitUnavailable", err.Error())
	}
	if err = assignJob(job, cmd.Process.Pid); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return nil, false, err
	}
	err = cmd.Wait()
	_ = windows.TerminateJobObject(job, 1)
	if ctx.Err() != nil {
		return nil, false, gitError("timeout", fmt.Sprintf("Git 操作超过 %s，请重试。", timeout))
	}
	if err != nil {
		code := cmd.ProcessState.ExitCode()
		ok := false
		for _, allowedCode := range allowed {
			if code == allowedCode {
				ok = true
			}
		}
		if !ok {
			message := strings.TrimSpace(diagnostic.String())
			if message == "" {
				message = err.Error()
			}
			return nil, false, gitError("git", message)
		}
	}
	return out.Bytes(), out.truncated, nil
}
func (g *GitReader) text(root string, args ...string) (string, error) {
	b, truncated, err := g.run(root, args, nil, gitReadLimit)
	if err == nil && truncated {
		err = gitError("tooLarge", "Git 返回内容超过读取上限，请缩小范围。")
	}
	return strings.TrimSpace(string(b)), err
}
func gitID(path string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(path)))
	return hex.EncodeToString(sum[:12])
}
func (g *GitReader) repo(id string) (gitRepo, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	r, ok := g.repos[id]
	if !ok {
		return r, gitError("invalid", "仓库已关闭，请重新打开。")
	}
	return r, nil
}
func (g *GitReader) CheckGit() (string, error) { return g.text("", "--version") }
func (g *GitReader) OpenRepository(path string) (GitProject, error) {
	var result GitProject
	abs, err := filepath.Abs(path)
	if err != nil {
		return result, err
	}
	abs, err = filepath.EvalSymlinks(abs)
	if err != nil {
		return result, err
	}
	root, err := g.text(abs, "rev-parse", "--show-toplevel")
	if err != nil {
		return result, err
	}
	root, err = filepath.EvalSymlinks(filepath.FromSlash(root))
	if err != nil {
		return result, err
	}
	bare, err := g.text(root, "rev-parse", "--is-bare-repository")
	if err != nil {
		return result, err
	}
	if bare == "true" {
		return result, gitError("unsupported", "暂不支持裸仓库。")
	}
	partial, _, err := g.run(root, []string{"config", "--get-regexp", `^(extensions\.partialclone|remote\..*\.promisor)$`}, nil, 32000, 1)
	if err != nil {
		return result, err
	}
	if len(bytes.TrimSpace(partial)) > 0 {
		return result, gitError("unsupported", "暂不读取部分克隆仓库，避免自动下载或修改对象库。")
	}
	gitDir, err := g.text(root, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return result, err
	}
	common, err := g.text(root, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return result, err
	}
	id := gitID(root)
	g.mu.Lock()
	g.repos[id] = gitRepo{root, gitDir, common}
	g.mu.Unlock()
	snapshot, err := g.GetSnapshot(id)
	if err != nil {
		g.mu.Lock()
		delete(g.repos, id)
		g.mu.Unlock()
		return result, err
	}
	return GitProject{id, snapshot}, nil
}
func (g *GitReader) checkFilters(root string) error {
	tracked, truncated, err := g.run(root, []string{"ls-files", "-z", "--cached", "--others", "--exclude-standard"}, nil, gitReadLimit)
	if err != nil {
		return err
	}
	if truncated {
		return gitError("tooLarge", "文件数量超过安全读取上限。")
	}
	if len(tracked) == 0 {
		return nil
	}
	filters := map[string]bool{}
	for _, cached := range []bool{false, true} {
		args := []string{"check-attr", "-z", "--stdin"}
		if cached {
			args = append(args, "--cached")
		}
		args = append(args, "filter")
		attrs, large, err := g.run(root, args, tracked, gitReadLimit)
		if err != nil {
			return err
		}
		if large {
			return gitError("tooLarge", "属性数量超过读取上限。")
		}
		parts := bytes.Split(attrs, []byte{0})
		for i := 2; i < len(parts); i += 3 {
			value := string(parts[i])
			if value != "unspecified" && value != "unset" && value != "set" && value != "" {
				filters[value] = true
			}
		}
	}
	for filter := range filters {
		if filter == "lfs" {
			continue
		}
		for _, kind := range []string{"clean", "process"} {
			value, _, err := g.run(root, []string{"config", "--get-all", "filter." + filter + "." + kind}, nil, 32000, 1)
			if err != nil {
				return err
			}
			if len(bytes.TrimSpace(value)) != 0 {
				return gitError("unsafeFilter", "仓库启用了外部 "+filter+" 过滤器，已停止读取以避免执行它。")
			}
		}
	}
	return nil
}
func (g *GitReader) refs(root string) ([]GitReference, []byte, error) {
	b, large, err := g.run(root, []string{"for-each-ref", "--format=%(refname)%00%(objectname)%00%(*objectname)%00", "refs/heads/", "refs/remotes/", "refs/tags/"}, nil, gitReadLimit)
	if err != nil {
		return nil, nil, err
	}
	if large {
		return nil, nil, gitError("tooLarge", "引用数量超过读取上限。")
	}
	refs := []GitReference{}
	for _, line := range bytes.Split(b, []byte{'\n'}) {
		p := bytes.Split(line, []byte{0})
		if len(p) < 3 {
			continue
		}
		full, hash := string(p[0]), string(p[1])
		if len(p[2]) > 0 {
			hash = string(p[2])
		}
		kind, name := "branch", strings.TrimPrefix(full, "refs/heads/")
		if strings.HasPrefix(full, "refs/remotes/") {
			kind = "remote"
			name = strings.TrimPrefix(full, "refs/remotes/")
		}
		if strings.HasPrefix(full, "refs/tags/") {
			kind = "tag"
			name = strings.TrimPrefix(full, "refs/tags/")
		}
		refs = append(refs, GitReference{name, full, hash, kind})
	}
	return refs, b, nil
}
func (g *GitReader) GetSnapshot(id string) (GitSnapshot, error) {
	r, err := g.repo(id)
	if err != nil {
		return GitSnapshot{}, err
	}
	s := GitSnapshot{Path: r.root, Name: filepath.Base(r.root), Files: []GitFileState{}, Refs: []GitReference{}, Worktrees: []GitWorktree{}, Stashes: []GitStash{}}
	if err = g.checkFilters(r.root); err != nil {
		return s, err
	}
	status, large, err := g.run(r.root, []string{"status", "--porcelain=v1", "-z", "--untracked-files=all", "--ignore-submodules=all"}, nil, gitReadLimit)
	if err != nil {
		return s, err
	}
	if large {
		return s, gitError("tooLarge", "仓库状态超过读取上限。")
	}
	parts := bytes.Split(status, []byte{0})
	for i := 0; i < len(parts); i++ {
		row := parts[i]
		if len(row) < 4 {
			continue
		}
		xy, path := string(row[:2]), string(row[3:])
		file := GitFileState{Path: path, XY: xy, Untracked: xy == "??"}
		file.Conflict = strings.Contains(xy, "U") || xy == "AA" || xy == "DD"
		file.Staged = !file.Untracked && !file.Conflict && xy[0] != ' '
		file.Unstaged = file.Untracked || (!file.Conflict && xy[1] != ' ')
		if strings.ContainsAny(xy, "RC") && i+1 < len(parts) {
			old := string(parts[i+1])
			file.OldPath = &old
			i++
		}
		s.Files = append(s.Files, file)
	}
	branch, _, err := g.run(r.root, []string{"symbolic-ref", "--quiet", "--short", "HEAD"}, nil, 32000, 1)
	if err != nil {
		return s, err
	}
	if v := strings.TrimSpace(string(branch)); v != "" {
		s.Branch = &v
	}
	head, _, err := g.run(r.root, []string{"rev-parse", "--verify", "HEAD"}, nil, 32000, 128)
	if err != nil {
		return s, err
	}
	if v := strings.TrimSpace(string(head)); validCommit(v) {
		s.Head = &v
	}
	refs, rawRefs, err := g.refs(r.root)
	if err != nil {
		return s, err
	}
	s.Refs = refs
	history := sha256.New()
	_, _ = history.Write(head)
	_, _ = history.Write(rawRefs)
	s.HistoryRevision = hex.EncodeToString(history.Sum(nil))
	changes := sha256.New()
	_, _ = changes.Write(status)
	if index, err := os.ReadFile(filepath.Join(r.gitDir, "index")); err == nil {
		_, _ = changes.Write(index)
	} else if !errors.Is(err, os.ErrNotExist) {
		return s, err
	}
	for _, file := range s.Files {
		p, err := safeGitFile(r.root, file.Path)
		if err != nil {
			continue
		}
		info, err := os.Stat(p)
		if err == nil {
			fmt.Fprintf(changes, "%s:%d:%d", file.Path, info.Size(), info.ModTime().UnixNano())
			if info.Mode().IsRegular() {
				f, e := os.Open(p)
				if e == nil {
					_, _ = io.Copy(changes, io.LimitReader(f, 256*1024))
					_ = f.Close()
				}
			}
		}
	}
	s.ChangesRevision = hex.EncodeToString(changes.Sum(nil))
	if s.Head != nil {
		upstream, _, e := g.run(r.root, []string{"rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}"}, nil, 32000, 128)
		if e == nil && len(bytes.TrimSpace(upstream)) > 0 {
			v := strings.TrimSpace(string(upstream))
			s.Upstream = &v
			counts, e := g.text(r.root, "rev-list", "--left-right", "--count", "HEAD...@{upstream}")
			if e == nil {
				fields := strings.Fields(counts)
				if len(fields) == 2 {
					s.Ahead, _ = strconv.Atoi(fields[0])
					s.Behind, _ = strconv.Atoi(fields[1])
				}
			}
		}
	}
	for _, operation := range []struct{ path, label string }{{"MERGE_HEAD", "merge"}, {"rebase-merge", "rebase"}, {"rebase-apply", "rebase"}, {"CHERRY_PICK_HEAD", "cherry-pick"}, {"REVERT_HEAD", "revert"}, {"BISECT_LOG", "bisect"}} {
		if _, e := os.Stat(filepath.Join(r.gitDir, operation.path)); e == nil {
			label := operation.label
			s.Operation = &label
			break
		}
	}
	worktrees, _, err := g.run(r.root, []string{"worktree", "list", "--porcelain", "-z"}, nil, gitReadLimit)
	if err != nil {
		return s, err
	}
	for _, block := range bytes.Split(worktrees, []byte{0, 0}) {
		w := GitWorktree{}
		for _, line := range bytes.Split(block, []byte{0}) {
			key, value, _ := strings.Cut(string(line), " ")
			switch key {
			case "worktree":
				w.Path = filepath.FromSlash(value)
			case "branch":
				w.Branch = strings.TrimPrefix(value, "refs/heads/")
			case "bare":
				w.Bare = true
			case "detached":
				w.Detached = true
			}
		}
		if w.Path != "" {
			w.ID = gitID(w.Path)
			w.Current = strings.EqualFold(filepath.Clean(w.Path), r.root)
			_, e := os.Stat(w.Path)
			w.Available = e == nil
			s.Worktrees = append(s.Worktrees, w)
		}
	}
	stashes, _, err := g.run(r.root, []string{"stash", "list", "-n100", "--format=%gd%x00%s%x00"}, nil, gitReadLimit)
	if err != nil {
		return s, err
	}
	for _, line := range bytes.Split(stashes, []byte{'\n'}) {
		p := bytes.Split(line, []byte{0})
		if len(p) >= 2 {
			s.Stashes = append(s.Stashes, GitStash{string(p[0]), string(p[1])})
		}
	}
	return s, nil
}

var commitPattern = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)

func validCommit(value string) bool { return commitPattern.MatchString(value) }
func parseGitCommits(raw []byte) []GitCommit {
	parts := bytes.Split(raw, []byte{0})
	result := []GitCommit{}
	for len(parts) >= 8 {
		hash := strings.TrimSpace(string(parts[0]))
		if !validCommit(hash) {
			break
		}
		result = append(result, GitCommit{Hash: hash, Parents: strings.Fields(string(parts[1])), Author: string(parts[2]), AuthorEmail: string(parts[3]), Committer: string(parts[4]), CommitterEmail: string(parts[5]), Date: string(parts[6]), Subject: string(parts[7])})
		parts = parts[8:]
	}
	return result
}
func (g *GitReader) GetHistory(id, reference string, offset int, expected string) (GitHistoryPage, error) {
	s, err := g.GetSnapshot(id)
	page := GitHistoryPage{Commits: []GitCommit{}, Offset: offset, Revision: s.HistoryRevision}
	if err != nil {
		return page, err
	}
	if expected != "" && expected != s.HistoryRevision {
		return page, gitError("changing", "仓库历史已变化，请刷新后重试。")
	}
	if s.Head == nil && len(s.Refs) == 0 {
		return page, nil
	}
	if offset < 0 || offset > 1000000 {
		return page, gitError("invalid", "历史分页参数无效。")
	}
	args := []string{"log", "--topo-order", "--date=iso-strict", "--format=" + gitCommitFormat, "--max-count=121", "--skip=" + strconv.Itoa(offset)}
	if reference == "all" || reference == "" {
		args = append(args, "--branches", "--remotes", "--tags")
		if s.Head != nil {
			args = append(args, "HEAD")
		}
	} else {
		valid := reference == "HEAD"
		for _, ref := range s.Refs {
			if reference == ref.FullName {
				valid = true
			}
		}
		if !valid {
			return page, gitError("invalid", "筛选引用不存在，请刷新。")
		}
		args = append(args, "--end-of-options", reference)
	}
	args = append(args, "--")
	raw, large, err := g.run(s.Path, args, nil, gitReadLimit)
	if err != nil {
		return page, err
	}
	if large {
		return page, gitError("tooLarge", "提交记录超过读取上限。")
	}
	page.Commits = parseGitCommits(raw)
	page.HasMore = len(page.Commits) > 120
	if page.HasMore {
		page.Commits = page.Commits[:120]
	}
	return page, nil
}
func safeGitFile(root, path string) (string, error) {
	if path == "" || !filepath.IsLocal(filepath.FromSlash(path)) {
		return "", gitError("invalid", "文件路径必须位于所选仓库内。")
	}
	joined := filepath.Join(root, filepath.FromSlash(path))
	parent := filepath.Dir(joined)
	for {
		info, err := os.Lstat(parent)
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			return "", gitError("invalid", "不跟随仓库目录中的符号链接。")
		}
		if strings.EqualFold(parent, root) {
			break
		}
		next := filepath.Dir(parent)
		if next == parent {
			return "", gitError("invalid", "无效仓库路径。")
		}
		parent = next
	}
	if info, err := os.Lstat(joined); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return "", gitError("unsupported", "符号链接仅显示仓库元信息，不读取其目标。")
	}
	return joined, nil
}

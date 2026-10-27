package trylet

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func gitFixture(t *testing.T) (*GitReader, string, string) {
	t.Helper()
	binary, err := exec.LookPath("git.exe")
	if err != nil {
		t.Skip("本机没有 Git")
	}
	root := filepath.Join(t.TempDir(), "仓库 空格")
	if err = os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	fixtureGit(t, root, "init", "--initial-branch=main")
	fixtureGit(t, root, "config", "user.name", "Trylet Test")
	fixtureGit(t, root, "config", "user.email", "local-test@example.invalid")
	fixtureGit(t, root, "config", "core.autocrlf", "false")
	data := t.TempDir()
	store, err := NewStore(data, data)
	if err != nil {
		t.Fatal(err)
	}
	reader := NewGitReader(store, t.TempDir(), nil)
	reader.binary = binary
	t.Cleanup(reader.Close)
	return reader, root, data
}
func fixtureGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git.exe", append([]string{"-C", root}, args...)...)
	for _, value := range os.Environ() {
		key, _, _ := strings.Cut(value, "=")
		if !strings.HasPrefix(strings.ToUpper(key), "GIT_") {
			cmd.Env = append(cmd.Env, value)
		}
	}
	cmd.Env = append(cmd.Env, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
	b, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("fixture git %v: %v %s", args, err, b)
	}
	return strings.TrimSpace(string(b))
}
func writeGitFixture(t *testing.T, root, path, content string) {
	t.Helper()
	name := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(name), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}
func gitFixtureHashes(t *testing.T, root string) map[string]string {
	t.Helper()
	result := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		relative, _ := filepath.Rel(root, path)
		result[relative] = fmt.Sprintf("%x:%d", sha256.Sum256(b), info.ModTime().UnixNano())
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}
func TestGitReadOnlyChangesHistoryAndDiff(t *testing.T) {
	g, root, _ := gitFixture(t)
	writeGitFixture(t, root, "中文 文件.py", "print('base')\n")
	fixtureGit(t, root, "add", ".")
	fixtureGit(t, root, "commit", "-m", "首次提交")
	first := fixtureGit(t, root, "rev-parse", "HEAD")
	writeGitFixture(t, root, "中文 文件.py", "print('staged')\n")
	fixtureGit(t, root, "add", ".")
	writeGitFixture(t, root, "中文 文件.py", "print('working')\n")
	writeGitFixture(t, root, "new file.txt", "new text\n")
	before := gitFixtureHashes(t, root)
	project, err := g.OpenRepository(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(project.Snapshot.Files) != 2 || project.Snapshot.Head == nil || *project.Snapshot.Head != first {
		t.Fatalf("快照错误 %+v", project.Snapshot)
	}
	var changed GitFileState
	for _, file := range project.Snapshot.Files {
		if file.Path == "中文 文件.py" {
			changed = file
		}
	}
	if !changed.Staged || !changed.Unstaged {
		t.Fatal("同一文件的暂存和工作区范围丢失")
	}
	for _, spec := range []struct{ mode, path, want string }{{"staged", "中文 文件.py", "+print('staged')"}, {"unstaged", "中文 文件.py", "+print('working')"}, {"unstaged", "new file.txt", "+new text"}} {
		diff, err := g.GetDiff(project.RepoID, spec.mode, spec.path, "", "", "")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(diff.Patch, spec.want) {
			t.Fatalf("差异范围 %s: %q", spec.mode, diff.Patch)
		}
	}
	history, err := g.GetHistory(project.RepoID, "all", 0, project.Snapshot.HistoryRevision)
	if err != nil || len(history.Commits) != 1 {
		t.Fatalf("历史错误 %v %+v", err, history)
	}
	view, err := g.GetCommitView(project.RepoID, first, "", project.Snapshot.HistoryRevision)
	if err != nil || view.Diff == nil || !strings.Contains(view.Diff.Patch, "+print('base')") {
		t.Fatalf("首次提交差异错误 %v %+v", err, view)
	}
	origin, err := g.GetLineOrigin(project.RepoID, GitLineRequest{Path: "中文 文件.py", Mode: "commit", Commit: first, Side: "after", Line: 1})
	if err != nil || origin.Author != "Trylet Test" || origin.Hash == nil {
		t.Fatalf("已提交行归属错误：%v %+v", err, origin)
	}
	for _, spec := range []struct {
		mode, path, side string
		committed        bool
	}{
		{"staged", "中文 文件.py", "before", true},
		{"staged", "中文 文件.py", "after", false},
		{"unstaged", "中文 文件.py", "before", false},
		{"unstaged", "new file.txt", "after", false},
	} {
		origin, err := g.GetLineOrigin(project.RepoID, GitLineRequest{Path: spec.path, Mode: spec.mode, Side: spec.side, Line: 1})
		if err != nil || (origin.Hash != nil) != spec.committed {
			t.Fatalf("%s/%s 行归属错误：%v %+v", spec.mode, spec.side, err, origin)
		}
	}
	after := gitFixtureHashes(t, root)
	if fmt.Sprint(before) != fmt.Sprint(after) {
		t.Fatal("只读查看修改了工作区或 .git 的内容/时间")
	}
}

func TestGitBinaryAndUTF16Diff(t *testing.T) {
	g, root, _ := gitFixture(t)
	encoded := func(value uint16) []byte {
		b := []byte{255, 254, 0, 0, 10, 0}
		binary.LittleEndian.PutUint16(b[2:4], value)
		return b
	}
	if err := os.WriteFile(filepath.Join(root, "utf16.txt"), encoded('A'), 0600); err != nil {
		t.Fatal(err)
	}
	makeImage := func(c color.RGBA) []byte {
		img := image.NewRGBA(image.Rect(0, 0, 2, 2))
		img.SetRGBA(0, 0, c)
		var b bytes.Buffer
		if err := png.Encode(&b, img); err != nil {
			t.Fatal(err)
		}
		return b.Bytes()
	}
	if err := os.WriteFile(filepath.Join(root, "sample.png"), makeImage(color.RGBA{R: 255, A: 255}), 0600); err != nil {
		t.Fatal(err)
	}
	fixtureGit(t, root, "add", ".")
	fixtureGit(t, root, "commit", "-m", "media base")
	if err := os.WriteFile(filepath.Join(root, "utf16.txt"), encoded('B'), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sample.png"), makeImage(color.RGBA{B: 255, A: 255}), 0600); err != nil {
		t.Fatal(err)
	}
	project, err := g.OpenRepository(root)
	if err != nil {
		t.Fatal(err)
	}
	text, err := g.GetDiff(project.RepoID, "unstaged", "utf16.txt", "", "", "")
	if err != nil || text.Binary || !strings.Contains(text.Patch, "+B") {
		t.Fatalf("UTF-16 比较失败：%v %+v", err, text)
	}
	media, err := g.GetDiff(project.RepoID, "unstaged", "sample.png", "", "", "")
	if err != nil || media.Preview == nil || media.Preview.Before == nil || media.Preview.After == nil || media.Preview.Before.DataURL == nil || media.Preview.After.Kind != "image" {
		t.Fatalf("媒体预览失败：%v %+v", err, media)
	}
}

func TestGitConflictDiff(t *testing.T) {
	g, root, _ := gitFixture(t)
	writeGitFixture(t, root, "conflict.txt", "base\n")
	fixtureGit(t, root, "add", ".")
	fixtureGit(t, root, "commit", "-m", "base")
	fixtureGit(t, root, "checkout", "-b", "side")
	writeGitFixture(t, root, "conflict.txt", "theirs\n")
	fixtureGit(t, root, "commit", "-am", "theirs")
	fixtureGit(t, root, "checkout", "main")
	writeGitFixture(t, root, "conflict.txt", "ours\n")
	fixtureGit(t, root, "commit", "-am", "ours")
	cmd := exec.Command("git.exe", "-C", root, "-c", "commit.gpgsign=false", "merge", "side")
	if b, err := cmd.CombinedOutput(); err == nil {
		t.Fatalf("测试仓库应产生冲突：%s", b)
	}
	project, err := g.OpenRepository(root)
	if err != nil {
		t.Fatal(err)
	}
	diff, err := g.GetDiff(project.RepoID, "conflict", "conflict.txt", "", "", "")
	if err != nil || diff.Conflict == nil || len(diff.Conflict.Blocks) != 1 {
		t.Fatalf("冲突块读取失败：%v %+v", err, diff)
	}
	block := diff.Conflict.Blocks[0]
	if len(block.Ours) != 1 || block.Ours[0].Text != "ours" || len(block.Theirs) != 1 || block.Theirs[0].Text != "theirs" {
		t.Fatalf("冲突方向错误 %+v", block)
	}
}
func TestGitEmptyRepositoryAndRename(t *testing.T) {
	g, root, _ := gitFixture(t)
	empty, err := g.OpenRepository(root)
	if err != nil {
		t.Fatal(err)
	}
	page, err := g.GetHistory(empty.RepoID, "all", 0, "")
	if err != nil || len(page.Commits) != 0 {
		t.Fatal("空仓库读取失败", err)
	}
	writeGitFixture(t, root, "before.txt", "hello rename\n")
	fixtureGit(t, root, "add", ".")
	fixtureGit(t, root, "commit", "-m", "base")
	fixtureGit(t, root, "mv", "before.txt", "after.txt")
	project, err := g.OpenRepository(root)
	if err != nil {
		t.Fatal(err)
	}
	diff, err := g.GetDiff(project.RepoID, "staged", "after.txt", "", "", "")
	if err != nil || diff.Binary {
		t.Fatal("暂存重命名读取失败", err)
	}
	writeGitFixture(t, root, "after.txt", "hello rename\nextra working line\n")
	working, err := g.GetDiff(project.RepoID, "unstaged", "after.txt", "", "", "")
	if err != nil || !strings.Contains(working.Patch, "+extra working line") || strings.Contains(working.Patch, "+hello rename") {
		t.Fatalf("暂存重命名后的未暂存差异基线错误：%v %q", err, working.Patch)
	}
	writeGitFixture(t, root, "after.txt", "hello rename\n")
	fixtureGit(t, root, "commit", "-m", "rename")
	hash := fixtureGit(t, root, "rev-parse", "HEAD")
	view, err := g.GetCommitView(project.RepoID, hash, "after.txt", "")
	if err != nil || len(view.Detail.Files) != 1 || view.Detail.Files[0].OldPath == nil || *view.Detail.Files[0].OldPath != "before.txt" {
		t.Fatalf("提交重命名读取失败 %v %+v", err, view)
	}
	if _, err = g.GetHistory(project.RepoID, "--output=bad", 0, ""); err == nil {
		t.Fatal("不应接受任意 Git 引用参数")
	}
	if _, err = g.GetDiff(project.RepoID, "unstaged", "../outside.txt", "", "", ""); err == nil {
		t.Fatal("不应读取仓库之外的路径")
	}
}
func TestGitRejectsActiveExternalFilterAndRoutingEnvironment(t *testing.T) {
	g, root, _ := gitFixture(t)
	writeGitFixture(t, root, "safe.txt", "safe\n")
	fixtureGit(t, root, "add", ".")
	fixtureGit(t, root, "commit", "-m", "base")
	t.Setenv("GIT_DIR", filepath.Join(t.TempDir(), "wrong.git"))
	if _, err := g.OpenRepository(root); err != nil {
		t.Fatal("继承的 GIT_DIR 改变了仓库路由", err)
	}
	writeGitFixture(t, root, ".gitattributes", "*.txt filter=unsafe\n")
	fixtureGit(t, root, "config", "filter.unsafe.clean", "echo MUST_NOT_RUN")
	_, err := g.OpenRepository(root)
	if err == nil || !strings.Contains(err.Error(), "unsafeFilter") {
		t.Fatal("应拒绝活动的外部过滤器", err)
	}
}
func TestGitLFSCleanAndObserver(t *testing.T) {
	var out bytes.Buffer
	if err := GitLFSClean(strings.NewReader("hello\n"), &out); err != nil {
		t.Fatal(err)
	}
	pointer := lfsPointer(out.Bytes())
	if pointer == nil || pointer.Size != 6 {
		t.Fatal("LFS 指针错误", out.String())
	}
	var again bytes.Buffer
	if err := GitLFSClean(bytes.NewReader(out.Bytes()), &again); err != nil || !bytes.Equal(out.Bytes(), again.Bytes()) {
		t.Fatal("LFS 指针重复过滤错误")
	}
	g, root, _ := gitFixture(t)
	project, err := g.OpenRepository(root)
	if err != nil {
		t.Fatal(err)
	}
	events := make(chan string, 10)
	g.emit = func(id string) {
		select {
		case events <- id:
		default:
		}
	}
	if err = g.ActivateRepository(project.RepoID, true); err != nil {
		t.Fatal(err)
	}
	g.SetVisible(true)
	g.SetActive(true)
	time.Sleep(150 * time.Millisecond)
	writeGitFixture(t, root, "watched.txt", "event\n")
	select {
	case id := <-events:
		if id != project.RepoID {
			t.Fatal("事件串仓库")
		}
	case <-time.After(4 * time.Second):
		t.Fatal("未收到目录变化事件")
	}
	g.SetActive(false)
	for len(events) > 0 {
		<-events
	}
	writeGitFixture(t, root, "watched.txt", "paused\n")
	select {
	case <-events:
		t.Fatal("隐藏 Git 页后仍收到监听通知")
	case <-time.After(350 * time.Millisecond):
	}
}

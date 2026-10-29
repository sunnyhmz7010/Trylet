package trylet

import (
	"bytes"
	"encoding/base64"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestRunnerInteractiveC(t *testing.T) {
	compiler, err := exec.LookPath("gcc.exe")
	if err != nil {
		t.Skip("本机没有 GCC")
	}
	events := make(chan RunEvent, 128)
	runner := NewRunner(t.TempDir(), func(e RunEvent) { events <- e })
	id := newID()
	code := "#include <stdio.h>\nint main(void){int n;printf(\"Number: \");scanf(\"%d\",&n);printf(\"answer=%d\\n\",n*2);return 0;}\n"
	_, err = runner.Start(RunRequest{ID: id, DraftID: newID(), Code: code, Language: "c", Cols: 80, Rows: 16}, RuntimeCandidate{Path: compiler, Kind: "gcc", Language: "c"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = runner.Stop(id) })
	output := ""
	inputSent := false
	timer := time.NewTimer(30 * time.Second)
	defer timer.Stop()
	for {
		select {
		case e := <-events:
			if e.Kind == "output" {
				b, _ := base64.StdEncoding.DecodeString(e.Data)
				output += string(b)
				if strings.Contains(output, "Number:") && !inputSent {
					if err = runner.Write(id, "21\r"); err != nil {
						t.Fatal(err)
					}
					inputSent = true
				}
			}
			if e.Kind == "exit" {
				if e.ExitCode != 0 || !strings.Contains(output, "answer=42") {
					t.Fatalf("%+v output=%q", e, output)
				}
				return
			}
		case <-timer.C:
			t.Fatalf("交互运行超时，output=%q", output)
		}
	}
}

func TestPortableStoreAndPublicationPreserved(t *testing.T) {
	root := t.TempDir()
	s, err := NewStore(root, root)
	if err != nil {
		t.Fatal(err)
	}
	d, err := s.Current()
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.SaveDraft(DraftInput{ID: d.ID, Code: "print(1)", Language: "python", Title: "练习"})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.SavePublication(d.ID, PublishRecord{SHA: "abc", Target: PublishTarget{Repository: "owner/repo", Path: "test.py", Branch: "main"}}); err != nil {
		t.Fatal(err)
	}
	result, err := s.SaveDraft(DraftInput{ID: d.ID, Code: "print(2)", Language: "python", Title: "练习"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Publish == nil || result.Publish.SHA != "abc" {
		t.Fatal("自动保存覆盖了发布记录")
	}
	settings := s.Settings()
	settings.Repository = "owner/repo"
	if err = s.SaveSettings(settings); err != nil {
		t.Fatal(err)
	}
	if err = s.SaveBallPosition(123, 234); err != nil {
		t.Fatal(err)
	}
	second, err := s.NewDraft()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.SaveDraft(DraftInput{ID: second.ID, Language: "c", Title: "第二份", Code: "int main(void){}"}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.LoadDraft(d.ID); err != nil {
		t.Fatal(err)
	}
	reloaded, err := NewStore(s.root, s.tempDir)
	if err != nil {
		t.Fatal(err)
	}
	current, err := reloaded.Current()
	if err != nil || current.Code != "print(2)" {
		t.Fatalf("恢复失败：%v %+v", err, current)
	}
	if len(reloaded.History()) != 2 || reloaded.Settings().Repository != "owner/repo" || reloaded.Settings().BallX != 123 {
		t.Fatal("同一数据文件未完整恢复历史与设置")
	}
	files, err := os.ReadDir(s.root)
	if err != nil || len(files) != 1 || files[0].Name() != "Trylet.json" {
		t.Fatalf("出现额外持久文件：%v %+v", err, files)
	}
	path := filepath.Join(s.root, "Trylet.json")
	if err = os.WriteFile(path, []byte("{broken"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = NewStore(s.root, s.tempDir); err == nil {
		t.Fatal("损坏的数据文件不应被静默重置")
	}
	b, err := os.ReadFile(path)
	if err != nil || string(b) != "{broken" {
		t.Fatal("损坏原文件被覆盖")
	}
}

func TestHistoryOrdersDraftsChronologically(t *testing.T) {
	store := &Store{state: diskState{Drafts: map[string]Draft{
		"older":  {ID: "older", UpdatedAt: "2026-10-29T14:00:00Z"},
		"newer":  {ID: "newer", UpdatedAt: "2026-10-29T14:00:00.1Z"},
		"latest": {ID: "latest", UpdatedAt: "2026-10-29T14:00:00.100000001Z"},
	}}}
	for i, draft := range store.History() {
		if expected := []string{"latest", "newer", "older"}[i]; draft.ID != expected {
			t.Fatalf("历史第 %d 项应为 %s，实际为 %s", i, expected, draft.ID)
		}
	}
}

func TestPortableStoreFailedWritePreservesState(t *testing.T) {
	root := t.TempDir()
	s, err := NewStore(root, root)
	if err != nil {
		t.Fatal(err)
	}
	d, err := s.Current()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(s.root, "Trylet.json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	name, _ := windows.UTF16PtrFromString(path)
	handle, err := windows.CreateFile(name, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, saveErr := s.SaveDraft(DraftInput{ID: d.ID, Language: "python", Title: "不应保存", Code: "print('failed')"})
	_ = windows.CloseHandle(handle)
	if saveErr == nil {
		t.Fatal("文件被占用时应明确返回保存错误")
	}
	current, err := s.Current()
	if err != nil || current.Code != d.Code || current.Title != d.Title {
		t.Fatal("写入失败仍改变了内存中的草稿")
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("写入失败破坏了上一份完整数据")
	}
	files, _ := os.ReadDir(s.root)
	if len(files) != 1 {
		t.Fatal("失败后遗留了临时保存文件")
	}
}

func TestPortableTokenEncryptionAndRemoval(t *testing.T) {
	root := t.TempDir()
	s, err := NewStore(root, root)
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(s, NewRunner(s.root, func(RunEvent) {}), nil)
	const fakeToken = "trylet-local-test-token-not-a-real-secret"
	if err = service.SetToken(fakeToken); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(s.root, "Trylet.json"))
	if err != nil || bytes.Contains(b, []byte(fakeToken)) || !s.hasToken() {
		t.Fatal("令牌应以密文保存在同级文件")
	}
	reloaded, err := NewStore(s.root, s.tempDir)
	if err != nil {
		t.Fatal(err)
	}
	if token, err := reloaded.readToken(); err != nil || token != fakeToken {
		t.Fatalf("重启后无法读取已保存令牌：%v", err)
	}
	reloaded.state.Token = []byte("invalid-encrypted-blob")
	if _, err = reloaded.readToken(); err == nil {
		t.Fatal("无效密文必须报错")
	}
	if _, err = NewService(reloaded, NewRunner(s.root, func(RunEvent) {}), nil).Bootstrap(); err != nil {
		t.Fatal("失效令牌不应阻止读取本地草稿")
	}
	if err = service.RemoveToken(); err != nil {
		t.Fatal(err)
	}
	reloaded, err = NewStore(s.root, s.tempDir)
	if err != nil || reloaded.hasToken() {
		t.Fatal("令牌移除后仍在持久文件中")
	}
	files, _ := os.ReadDir(s.root)
	if len(files) != 1 || files[0].Name() != "Trylet.json" {
		t.Fatal("令牌保存生成了额外文件")
	}
}

func TestRunnerUsesPortableTempDirectory(t *testing.T) {
	var candidate RuntimeCandidate
	for _, found := range installedRuntimeCandidates() {
		if found.Language == "python" {
			candidate = found
			break
		}
	}
	if candidate.Path == "" {
		t.Skip("本机没有可用的 Python 3")
	}
	outside := filepath.Join(t.TempDir(), "outside-must-stay-absent")
	candidate.Env = append(os.Environ(), "TEMP="+outside, "TMP="+outside, "TMPDIR="+outside, "PYTHONDONTWRITEBYTECODE=0")
	source := "import os, sys, tempfile\nassert os.path.samefile(tempfile.gettempdir(), os.getcwd())\nassert os.environ['TMP'] == os.getcwd()\nassert sys.dont_write_bytecode\nwith open('portable_module.py', 'w', encoding='utf-8') as f: f.write('answer = 42\\n')\nimport portable_module\nassert not os.path.exists('__pycache__')\nwith tempfile.TemporaryFile() as f: f.write(b'local')\nprint('portable-temp-ok')\n"
	runInteractiveFixture(t, candidate, source, "", "", "portable-temp-ok")
	if _, err := os.Stat(outside); !os.IsNotExist(err) {
		t.Fatal("使用了候选环境携带的外部临时目录")
	}
}

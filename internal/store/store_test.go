package store

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"

	publication "trylet/internal/publish"
)

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
	if err = s.SavePublication(d.ID, publication.PublishRecord{SHA: "abc", Target: publication.PublishTarget{Repository: "owner/repo", Path: "test.py", Branch: "main"}}); err != nil {
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
	if err != nil || len(files) != 2 || files[0].Name() != "Trylet.backup.json" || files[1].Name() != "Trylet.json" {
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
	for _, file := range files {
		if file.Name() != "Trylet.json" && file.Name() != "Trylet.backup.json" {
			t.Fatal("失败后遗留了临时保存文件")
		}
	}
}

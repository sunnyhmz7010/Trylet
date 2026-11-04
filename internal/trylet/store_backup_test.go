package trylet

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestStoreBackupRecoveryPreservesDamagedData(t *testing.T) {
	root := t.TempDir()
	store, err := NewStore(root, root)
	if err != nil {
		t.Fatal(err)
	}
	draft, err := store.Current()
	if err != nil {
		t.Fatal(err)
	}
	input := DraftInput{ID: draft.ID, Language: "python", Title: "备份测试", Code: "print('A')\r\n"}
	first, err := store.SaveDraft(input)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "Trylet.json")
	expected, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if unchanged, err := store.SaveDraft(input); err != nil || unchanged.UpdatedAt != first.UpdatedAt {
		t.Fatalf("未修改的草稿不应重写数据：%+v, %v", unchanged, err)
	}
	input.Code = "print('B')\n"
	if _, err := store.SaveDraft(input); err != nil {
		t.Fatal(err)
	}
	damaged := []byte(`{"version":1,"drafts":`)
	if err := os.WriteFile(path, damaged, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewStore(root, root); err == nil || !CanRestoreStoreBackup(root) {
		t.Fatal("损坏数据应保留并提供有效备份恢复")
	}
	preserved, err := RestoreStoreBackup(root)
	if err != nil {
		t.Fatal(err)
	}
	skillTestRead(t, preserved, string(damaged))
	skillTestRead(t, path, string(expected))
	skillTestRead(t, filepath.Join(root, "Trylet.backup.json"), string(expected))
	reopened, err := NewStore(root, root)
	if err != nil {
		t.Fatal(err)
	}
	got, err := reopened.Current()
	if err != nil || got.Code != "print('A')\n" {
		t.Fatalf("未恢复上一次完整草稿：%+v, %v", got, err)
	}
	exported := filepath.Join(root, "practice.py")
	if err := reopened.ExportDraft(draft.ID, exported); err != nil {
		t.Fatal(err)
	}
	skillTestRead(t, exported, got.Code)
	if _, err := RestoreStoreBackup(root); err == nil {
		t.Fatal("不能用恢复覆盖仍然完整的数据文件")
	}
}

func TestStoreBackupFailureKeepsCurrentData(t *testing.T) {
	root := t.TempDir()
	store, err := NewStore(root, root)
	if err != nil {
		t.Fatal(err)
	}
	draft, err := store.Current()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "Trylet.json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "Trylet.backup.json"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SaveDraft(DraftInput{ID: draft.ID, Language: "python", Code: "print('failed')"}); err == nil {
		t.Fatal("备份失败不能伪装保存成功")
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("备份失败改变了主数据文件")
	}
	got, err := store.Current()
	if err != nil || got.Code != draft.Code {
		t.Fatal("备份失败改变了内存中的草稿")
	}
}

func TestStoreRejectsInvalidBackupAndFutureVersion(t *testing.T) {
	for _, backup := range []string{`{`, `{"version":2}`, `{"version":1,"current":"missing"}`} {
		t.Run(backup, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "Trylet.json")
			if err := os.WriteFile(path, []byte("damaged"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "Trylet.backup.json"), []byte(backup), 0600); err != nil {
				t.Fatal(err)
			}
			if CanRestoreStoreBackup(root) {
				t.Fatal("无效备份不应提供恢复")
			}
			if _, err := RestoreStoreBackup(root); err == nil {
				t.Fatal("无效备份不应替换原数据")
			}
			skillTestRead(t, path, "damaged")
		})
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "Trylet.json"), []byte(`{"version":2}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Trylet.backup.json"), []byte(`{"version":1}`), 0600); err != nil {
		t.Fatal(err)
	}
	if CanRestoreStoreBackup(root) {
		t.Fatal("较新版本的数据不能被旧备份自动降级")
	}
}

func TestStoreBackupDoesNotRetainCredentials(t *testing.T) {
	root := t.TempDir()
	store, err := NewStore(root, root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Current(); err != nil {
		t.Fatal(err)
	}
	if err := store.saveToken("local-test-token"); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveBallPosition(10, 20); err != nil {
		t.Fatal(err)
	}
	if err := store.saveToken(""); err != nil {
		t.Fatal(err)
	}
	backup, err := os.ReadFile(filepath.Join(root, "Trylet.backup.json"))
	if err != nil {
		t.Fatal(err)
	}
	var saved diskState
	if err := json.Unmarshal(backup, &saved); err != nil || len(saved.Token) != 0 {
		t.Fatal("备份不应保留已移除的访问令牌")
	}
}

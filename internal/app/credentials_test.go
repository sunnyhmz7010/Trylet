package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	execution "trylet/internal/runner"
	storage "trylet/internal/store"
)

func TestPortableTokenEncryptionAndRemoval(t *testing.T) {
	root := t.TempDir()
	s, err := storage.NewStore(root, root)
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(s, execution.NewRunner(s.Root(), func(execution.RunEvent) {}), nil)
	const fakeToken = "trylet-local-test-token-not-a-real-secret"
	if err = service.SetToken(fakeToken); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(s.Root(), "Trylet.json"))
	if err != nil || bytes.Contains(b, []byte(fakeToken)) || !s.HasToken() {
		t.Fatal("令牌应以密文保存在同级文件")
	}
	reloaded, err := storage.NewStore(s.Root(), s.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if token, err := reloaded.ReadToken(); err != nil || token != fakeToken {
		t.Fatalf("重启后无法读取已保存令牌：%v", err)
	}
	var saved map[string]json.RawMessage
	if err = json.Unmarshal(b, &saved); err != nil {
		t.Fatal(err)
	}
	saved["encryptedToken"], err = json.Marshal([]byte("invalid-encrypted-blob"))
	if err != nil {
		t.Fatal(err)
	}
	damaged, err := json.Marshal(saved)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, "Trylet.json"), damaged, 0600); err != nil {
		t.Fatal(err)
	}
	reloaded, err = storage.NewStore(root, root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = reloaded.ReadToken(); err == nil {
		t.Fatal("无效密文必须报错")
	}
	if _, err = NewService(reloaded, execution.NewRunner(s.Root(), func(execution.RunEvent) {}), nil).Bootstrap(); err != nil {
		t.Fatal("失效令牌不应阻止读取本地草稿")
	}
	if err = service.RemoveToken(); err != nil {
		t.Fatal(err)
	}
	reloaded, err = storage.NewStore(s.Root(), s.TempDir())
	if err != nil || reloaded.HasToken() {
		t.Fatal("令牌移除后仍在持久文件中")
	}
	files, _ := os.ReadDir(s.Root())
	if len(files) != 2 || files[0].Name() != "Trylet.backup.json" || files[1].Name() != "Trylet.json" {
		t.Fatal("令牌保存生成了额外文件")
	}
}

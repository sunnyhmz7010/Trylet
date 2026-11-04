package app

import (
	"os"
	"path/filepath"
	"testing"

	storage "trylet/internal/store"
)

func TestGlobalDisplayPreferencesPersist(t *testing.T) {
	root := t.TempDir()
	store, err := storage.NewStore(root, root)
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(store, nil, nil, nil)
	settings := store.Settings()
	settings.Theme = "dark"
	settings.GitDiffLayout = "unified"
	if _, err := service.SavePreferences(settings, ""); err != nil {
		t.Fatal(err)
	}
	reopened, err := storage.NewStore(root, root)
	if err != nil {
		t.Fatal(err)
	}
	if got := reopened.Settings(); got.Theme != "dark" || got.GitDiffLayout != "unified" {
		t.Fatalf("重启后未恢复全局偏好：%+v", got)
	}
	for _, change := range []func(*storage.Settings){
		func(s *storage.Settings) { s.Theme = "green" },
		func(s *storage.Settings) { s.GitDiffLayout = "invalid" },
	} {
		invalid := settings
		change(&invalid)
		if err := service.SaveSettings(invalid); err == nil {
			t.Fatal("无效偏好未被拒绝")
		}
		if got := store.Settings(); got.Theme != "dark" || got.GitDiffLayout != "unified" {
			t.Fatal("保存失败覆盖了原偏好")
		}
	}
}

func TestWindowPreferencesPersist(t *testing.T) {
	root := t.TempDir()
	// 旧数据没有记忆标记时，首次关闭仍需选择，并沿用悬浮窗默认值。
	if err := os.WriteFile(filepath.Join(root, "Trylet.json"), []byte(`{"version":1,"settings":{"theme":"dark"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	store, err := storage.NewStore(root, root)
	if err != nil {
		t.Fatal(err)
	}
	settings := store.Settings()
	if settings.CloseAction != "background" || settings.RememberCloseAction || !settings.BallEnabled || !settings.BallFollowWindow || settings.Theme != "dark" {
		t.Fatalf("旧数据未补齐窗口默认值：%+v", settings)
	}
	service := NewService(store, nil, nil, nil)
	if err := store.RememberCloseAction("quit"); err != nil {
		t.Fatal(err)
	}
	settings = store.Settings()
	if settings.Theme != "dark" || !settings.BallEnabled || !settings.BallFollowWindow {
		t.Fatalf("记住关闭方式覆盖了其他偏好：%+v", settings)
	}
	settings.BallEnabled = false
	settings.BallFollowWindow = false
	if err := service.SaveSettings(settings); err != nil {
		t.Fatal(err)
	}
	reopened, err := storage.NewStore(root, root)
	if err != nil {
		t.Fatal(err)
	}
	if got := reopened.Settings(); got.CloseAction != "quit" || !got.RememberCloseAction || got.BallEnabled || got.BallFollowWindow {
		t.Fatalf("重启后未恢复窗口偏好：%+v", got)
	}
	invalid := settings
	invalid.CloseAction = "invalid"
	if err := service.SaveSettings(invalid); err == nil {
		t.Fatal("无效窗口关闭方式未被拒绝")
	}
	if err := store.RememberCloseAction("invalid"); err == nil {
		t.Fatal("记住了无效窗口关闭方式")
	}
	if got := store.Settings(); got.CloseAction != "quit" || !got.RememberCloseAction || got.BallEnabled || got.BallFollowWindow {
		t.Fatal("保存失败覆盖了原窗口偏好")
	}
	settings.RememberCloseAction = false
	if err := service.SaveSettings(settings); err != nil {
		t.Fatal(err)
	}
	reopened, err = storage.NewStore(root, root)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.Settings().RememberCloseAction {
		t.Fatal("未能恢复每次询问")
	}
}

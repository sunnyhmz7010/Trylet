package trylet

import (
	"testing"
)

func TestGlobalDisplayPreferencesPersist(t *testing.T) {
	root := t.TempDir()
	store, err := NewStore(root, root)
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(store, nil, nil)
	settings := store.Settings()
	settings.Theme = "dark"
	settings.GitDiffLayout = "unified"
	if _, err := service.SavePreferences(settings, ""); err != nil {
		t.Fatal(err)
	}
	reopened, err := NewStore(root, root)
	if err != nil {
		t.Fatal(err)
	}
	if got := reopened.Settings(); got.Theme != "dark" || got.GitDiffLayout != "unified" {
		t.Fatalf("重启后未恢复全局偏好：%+v", got)
	}
	for _, change := range []func(*Settings){
		func(s *Settings) { s.Theme = "green" },
		func(s *Settings) { s.GitDiffLayout = "invalid" },
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

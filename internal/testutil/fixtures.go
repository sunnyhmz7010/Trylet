package testutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func Repository(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git.exe"); err != nil {
		t.Skip("本机没有 Git")
	}
	root := filepath.Join(t.TempDir(), "仓库 空格")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	Git(t, root, "init", "--initial-branch=main")
	Git(t, root, "config", "user.name", "Trylet Test")
	Git(t, root, "config", "user.email", "local-test@example.invalid")
	Git(t, root, "config", "core.autocrlf", "false")
	return root
}

func WriteFile(t *testing.T, root, path, content string) {
	t.Helper()
	name := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(name), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

func Git(t *testing.T, root string, args ...string) string {
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

func ReadFile(t *testing.T, path, expected string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil || string(data) != expected {
		t.Fatalf("文件 %s：got %q, err %v, want %q", path, data, err, expected)
	}
}

package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestPortableRuntimeEnvironmentAndOwnership(t *testing.T) {
	root := t.TempDir()
	data := filepath.Join(root, "Trylet.json")
	if err := os.WriteFile(data, []byte("preserve"), 0600); err != nil {
		t.Fatal(err)
	}
	first, err := prepareRuntime(root)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, runtimeFolder)
	for _, key := range []string{"TEMP", "TMP", "TMPDIR", "WEBVIEW2_USER_DATA_FOLDER"} {
		t.Setenv(key, os.Getenv(key))
	}
	if err = setRuntimeEnvironment(dir); err != nil || os.TempDir() != dir || os.Getenv("WEBVIEW2_USER_DATA_FOLDER") != filepath.Join(dir, "webview2") {
		t.Fatal("临时目录或 WebView2 未固定在 EXE 同级", err)
	}
	if err = os.WriteFile(filepath.Join(dir, ".Trylet-save-interrupted"), []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	second, err := prepareRuntime(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(dir, ".Trylet-save-interrupted")); !os.IsNotExist(err) {
		t.Fatal("下次启动未清理中断的保存临时文件")
	}
	if err = withRuntimeLock(root, func() error { return removeRuntime(root, first) }); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(dir); err != nil {
		t.Fatal("旧进程删除了新会话的工作目录")
	}
	if err = withRuntimeLock(root, func() error { return removeRuntime(root, second) }); err != nil {
		t.Fatal(err)
	}
	files, _ := os.ReadDir(root)
	if len(files) != 1 || files[0].Name() != "Trylet.json" {
		t.Fatal("清理后应只保留同级数据文件")
	}
}

func TestPortableRuntimePreservesUnownedPath(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, runtimeFolder)
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "user-file")
	if err := os.WriteFile(path, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := prepareRuntime(root); err == nil {
		t.Fatal("不应清理没有本应用归属标记的目录")
	}
	if b, err := os.ReadFile(path); err != nil || string(b) != "keep" {
		t.Fatal("其他目录内容被改动")
	}
}

func TestPortableCleanupWaitsForReleasedFiles(t *testing.T) {
	root := t.TempDir()
	owner, err := prepareRuntime(root)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, runtimeFolder)
	name, _ := windows.UTF16PtrFromString(filepath.Join(dir, "locked-cache"))
	handle, err := windows.CreateFile(name, windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil, windows.CREATE_ALWAYS, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = windows.CloseHandle(handle) })
	done := make(chan error, 1)
	go func() { done <- withRuntimeLock(root, func() error { return removeRuntime(root, owner) }) }()
	time.Sleep(150 * time.Millisecond)
	if _, err = os.Stat(filepath.Join(dir, ".owner")); err != nil {
		t.Fatal("文件仍被占用时丢失了恢复标记")
	}
	_ = windows.CloseHandle(handle)
	select {
	case err = <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(12 * time.Second):
		t.Fatal("文件释放后未完成清理")
	}
	if _, err = os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("文件释放后仍遗留临时目录")
	}
}

func TestPortableCleanupWaitsForParentExit(t *testing.T) {
	root := t.TempDir()
	owner, err := prepareRuntime(root)
	if err != nil {
		t.Fatal(err)
	}
	data := filepath.Join(root, "Trylet.json")
	if err = os.WriteFile(data, []byte("preserve"), 0600); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	child := func(mode string, extra ...string) *exec.Cmd {
		cmd := exec.Command(executable, "-test.run=^TestPortableSubprocess$")
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
		cmd.Env = append(os.Environ(), "TRYLET_TEST_MODE="+mode, "TRYLET_TEST_ROOT="+root, "TRYLET_TEST_OWNER="+owner)
		cmd.Env = append(cmd.Env, extra...)
		return cmd
	}
	parent := child("parent")
	stdout, err := parent.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = parent.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = parent.Process.Kill() })
	ready := make(chan bool, 1)
	go func() { ready <- bufio.NewScanner(stdout).Scan() }()
	select {
	case ok := <-ready:
		if !ok {
			t.Fatal("测试父进程未就绪")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("等待测试父进程超时")
	}
	cleanup := child("cleanup", "TRYLET_TEST_PID="+strconv.Itoa(parent.Process.Pid))
	if err = cleanup.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cleanup.Process.Kill() })
	time.Sleep(150 * time.Millisecond)
	if _, err = os.Stat(filepath.Join(root, runtimeFolder)); err != nil {
		t.Fatal("父进程仍存活时提前清理了目录")
	}
	_ = parent.Process.Kill()
	_ = parent.Wait()
	if err = cleanup.Wait(); err != nil {
		t.Fatal("退出后清理失败", err)
	}
	if _, err = os.Stat(filepath.Join(root, runtimeFolder)); !os.IsNotExist(err) {
		t.Fatal("工作目录未删除", err)
	}
	if b, err := os.ReadFile(data); err != nil || string(b) != "preserve" {
		t.Fatal("清理影响了持久数据文件")
	}
}

func TestPortableSubprocess(t *testing.T) {
	root := os.Getenv("TRYLET_TEST_ROOT")
	switch os.Getenv("TRYLET_TEST_MODE") {
	case "parent":
		name, _ := windows.UTF16PtrFromString(filepath.Join(root, runtimeFolder, "locked-cache"))
		handle, err := windows.CreateFile(name, windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil, windows.CREATE_ALWAYS, windows.FILE_ATTRIBUTE_NORMAL, 0)
		if err != nil {
			t.Fatal(err)
		}
		defer windows.CloseHandle(handle)
		fmt.Println("ready")
		for {
			time.Sleep(time.Second)
		}
	case "cleanup":
		if err := runCleanupMode(root, []string{os.Getenv("TRYLET_TEST_PID"), os.Getenv("TRYLET_TEST_OWNER")}); err != nil {
			t.Fatal(err)
		}
	}
}

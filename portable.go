package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
)

const runtimeFolder = ".Trylet-runtime"

func withRuntimeLock(root string, action func() error) error {
	// Windows mutex 的所有权属于线程，获取与释放必须保持在同一 OS 线程。
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	hash := sha256.Sum256([]byte(strings.ToLower(filepath.Clean(root))))
	name, err := windows.UTF16PtrFromString(fmt.Sprintf(`Local\Trylet.files.%x`, hash[:12]))
	if err != nil {
		return err
	}
	handle, err := windows.CreateMutex(nil, false, name)
	if err != nil && !errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		return err
	}
	defer windows.CloseHandle(handle)
	result, err := windows.WaitForSingleObject(handle, 15000)
	if err != nil {
		return err
	}
	if result != windows.WAIT_OBJECT_0 && result != windows.WAIT_ABANDONED {
		return errors.New("另一进程仍在清理本应用的临时目录，请稍后重试")
	}
	defer windows.ReleaseMutex(handle)
	return action()
}

func prepareRuntime(root string) (string, error) {
	var id [12]byte
	if _, err := rand.Read(id[:]); err != nil {
		return "", err
	}
	owner := hex.EncodeToString(id[:])
	err := withRuntimeLock(root, func() error {
		dir := filepath.Join(root, runtimeFolder)
		if info, err := os.Lstat(dir); err == nil {
			if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
				return errors.New("同级运行路径被其他文件或链接占用，已保留原内容")
			}
			previous, err := os.ReadFile(filepath.Join(dir, ".owner"))
			if err != nil {
				return fmt.Errorf("临时目录缺少本应用的归属标记，已保留原内容：%w", err)
			}
			if err = removeRuntime(root, string(previous)); err != nil {
				return err
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := os.Mkdir(dir, 0700); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, ".owner"), []byte(owner), 0600); err != nil {
			_ = os.Remove(dir)
			return err
		}
		return nil
	})
	return owner, err
}

func removeRuntime(root, owner string) error {
	dir := filepath.Join(root, runtimeFolder)
	info, err := os.Lstat(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("运行路径已变成其他文件或链接，取消清理")
	}
	marker, err := os.ReadFile(filepath.Join(dir, ".owner"))
	if err != nil {
		return err
	}
	if string(marker) != owner {
		return nil
	}
	if id, err := hex.DecodeString(owner); err != nil || len(id) != 12 {
		return errors.New("临时目录归属标记无效，取消清理")
	}
	// 浏览器可能在宿主退出后才释放文件；只重试这个固定的同级目录。
	for deadline := time.Now().Add(10 * time.Second); ; {
		entries, readErr := os.ReadDir(dir)
		if readErr != nil {
			return readErr
		}
		err = nil
		for _, entry := range entries {
			if entry.Name() != ".owner" {
				err = errors.Join(err, os.RemoveAll(filepath.Join(dir, entry.Name())))
			}
		}
		if err == nil {
			if err = os.Remove(filepath.Join(dir, ".owner")); err != nil {
				return err
			}
			if err = os.Remove(dir); err == nil {
				return nil
			}
			if markerErr := os.WriteFile(filepath.Join(dir, ".owner"), marker, 0600); markerErr != nil {
				return errors.Join(err, markerErr)
			}
		}
		if time.Now().After(deadline) {
			return err
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func setRuntimeEnvironment(dir string) error {
	for _, key := range []string{"TEMP", "TMP", "TMPDIR"} {
		if err := os.Setenv(key, dir); err != nil {
			return err
		}
	}
	return os.Setenv("WEBVIEW2_USER_DATA_FOLDER", filepath.Join(dir, "webview2"))
}

func startCleanup(executable, owner string) error {
	command := exec.Command(executable, "--trylet-cleanup", strconv.Itoa(os.Getpid()), owner)
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	if err := command.Start(); err != nil {
		return err
	}
	return command.Process.Release()
}

func runCleanupMode(root string, args []string) error {
	if len(args) != 2 {
		return errors.New("清理参数无效")
	}
	pid, err := strconv.Atoi(args[0])
	owner, decodeErr := hex.DecodeString(args[1])
	if err != nil || pid <= 0 || decodeErr != nil || len(owner) != 12 {
		return errors.New("清理参数无效")
	}
	parent, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err == nil {
		result, waitErr := windows.WaitForSingleObject(parent, 30000)
		_ = windows.CloseHandle(parent)
		if waitErr != nil {
			return waitErr
		}
		if result != windows.WAIT_OBJECT_0 {
			return errors.New("主进程尚未结束，保留临时目录")
		}
	} else if !errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
		return err
	}
	return withRuntimeLock(root, func() error { return removeRuntime(root, args[1]) })
}

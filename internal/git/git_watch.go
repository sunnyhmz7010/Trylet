package git

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/windows"

	storage "trylet/internal/store"
)

func (g *GitReader) RecentRepositories() []storage.GitRecent {
	return g.store.RecentRepositories()
}

func (g *GitReader) ActivateRepository(id string, remember bool) error {
	r, err := g.repo(id)
	if err != nil {
		return err
	}
	g.mu.Lock()
	g.active = id
	g.restartWatch()
	g.mu.Unlock()
	if !remember {
		return nil
	}
	return g.store.RememberGitRepository(r.root)
}

func (g *GitReader) ForgetRecentRepository(path string) ([]storage.GitRecent, error) {
	return g.store.ForgetGitRepository(path)
}

func (g *GitReader) Preferences() map[string]string {
	return g.store.GitPreferences()
}

func (g *GitReader) SetPreference(key, value string) error {
	if !strings.HasPrefix(key, "oil-git.") || len(key) > 100 || len(value) > 1024 {
		return gitError("invalid", "偏好设置无效。")
	}
	return g.store.SaveGitPreference(key, value)
}

func (g *GitReader) SetActive(active bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.activeTab = active
	g.restartWatch()
}

func (g *GitReader) SetVisible(visible bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.visible = visible
	g.restartWatch()
}

func (g *GitReader) Close() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.stopWatch != nil {
		g.stopWatch()
		g.stopWatch = nil
	}
}

func (g *GitReader) restartWatch() {
	if g.stopWatch != nil {
		g.stopWatch()
		g.stopWatch = nil
	}
	if !g.activeTab || !g.visible || g.active == "" {
		return
	}
	r, ok := g.repos[g.active]
	if !ok {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	g.stopWatch = cancel
	id := g.active
	paths := []string{r.root}
	for _, extra := range []string{r.gitDir, r.commonDir} {
		covered := false
		for _, parent := range paths {
			relative, err := filepath.Rel(parent, extra)
			if err == nil && (relative == "." || filepath.IsLocal(relative)) {
				covered = true
			}
		}
		if !covered {
			paths = append(paths, extra)
		}
	}
	var mu sync.Mutex
	var timer *time.Timer
	notify := func() {
		mu.Lock()
		defer mu.Unlock()
		if ctx.Err() != nil {
			return
		}
		if timer != nil {
			timer.Stop()
		}
		timer = time.AfterFunc(180*time.Millisecond, func() {
			if ctx.Err() == nil && g.emit != nil {
				g.emit(id)
			}
		})
	}
	context.AfterFunc(ctx, func() {
		mu.Lock()
		defer mu.Unlock()
		if timer != nil {
			timer.Stop()
		}
	})
	for _, path := range paths {
		go g.observe(ctx, path, notify)
	}
}

func (g *GitReader) observe(ctx context.Context, path string, notify func()) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return
	}
	handle, err := windows.CreateFile(name, windows.FILE_LIST_DIRECTORY, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OVERLAPPED, 0)
	if err != nil {
		notify()
		return
	}
	defer windows.CloseHandle(handle)
	event, err := windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		notify()
		return
	}
	defer windows.CloseHandle(event)
	cancelled := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { defer close(cancelled); _ = windows.CancelIoEx(handle, nil) })
	defer func() {
		if !stop() {
			<-cancelled
		}
	}()
	buffer := make([]byte, 64*1024)
	flags := uint32(windows.FILE_NOTIFY_CHANGE_FILE_NAME | windows.FILE_NOTIFY_CHANGE_DIR_NAME | windows.FILE_NOTIFY_CHANGE_SIZE | windows.FILE_NOTIFY_CHANGE_LAST_WRITE)
	for ctx.Err() == nil {
		_ = windows.ResetEvent(event)
		overlapped := windows.Overlapped{HEvent: event}
		var count uint32
		err = windows.ReadDirectoryChanges(handle, &buffer[0], uint32(len(buffer)), true, flags, &count, &overlapped, 0)
		if err != nil && !errors.Is(err, windows.ERROR_IO_PENDING) {
			return
		}
		if ctx.Err() != nil {
			_ = windows.CancelIoEx(handle, &overlapped)
		}
		_, err = windows.WaitForSingleObject(event, windows.INFINITE)
		if err != nil || ctx.Err() != nil {
			return
		}
		if err = windows.GetOverlappedResult(handle, &overlapped, &count, false); err != nil && !errors.Is(err, windows.ERROR_NOTIFY_ENUM_DIR) {
			return
		}
		notify()
	}
}

package trylet

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/windows"
)

func (g *GitReader) RecentRepositories() []GitRecent {
	g.store.mu.Lock()
	defer g.store.mu.Unlock()
	return append([]GitRecent{}, g.store.state.GitRecent...)
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
	g.store.mu.Lock()
	old := g.store.state.GitRecent
	recent := []GitRecent{{Path: r.root, Name: filepath.Base(r.root)}}
	for _, item := range old {
		if !strings.EqualFold(item.Path, r.root) {
			recent = append(recent, item)
		}
	}
	if len(recent) > 12 {
		recent = recent[:12]
	}
	g.store.state.GitRecent = recent
	err = g.store.save()
	if err != nil {
		g.store.state.GitRecent = old
	}
	g.store.mu.Unlock()
	if err != nil {
		return err
	}
	return nil
}
func (g *GitReader) ForgetRecentRepository(path string) ([]GitRecent, error) {
	g.store.mu.Lock()
	defer g.store.mu.Unlock()
	old := g.store.state.GitRecent
	next := []GitRecent{}
	for _, r := range old {
		if !strings.EqualFold(r.Path, path) {
			next = append(next, r)
		}
	}
	g.store.state.GitRecent = next
	if err := g.store.save(); err != nil {
		g.store.state.GitRecent = old
		return nil, err
	}
	return append([]GitRecent{}, next...), nil
}
func (g *GitReader) Preferences() map[string]string {
	g.store.mu.Lock()
	defer g.store.mu.Unlock()
	result := map[string]string{}
	for key, value := range g.store.state.GitPreferences {
		result[key] = value
	}
	return result
}
func (g *GitReader) SetPreference(key, value string) error {
	if !strings.HasPrefix(key, "oil-git.") || len(key) > 100 || len(value) > 1024 {
		return gitError("invalid", "偏好设置无效。")
	}
	g.store.mu.Lock()
	defer g.store.mu.Unlock()
	if g.store.state.GitPreferences == nil {
		g.store.state.GitPreferences = map[string]string{}
	}
	previous, existed := g.store.state.GitPreferences[key]
	g.store.state.GitPreferences[key] = value
	if err := g.store.save(); err != nil {
		if existed {
			g.store.state.GitPreferences[key] = previous
		} else {
			delete(g.store.state.GitPreferences, key)
		}
		return err
	}
	return nil
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

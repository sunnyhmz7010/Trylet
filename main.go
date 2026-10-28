package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"log/slog"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/w32"
	"golang.org/x/sys/windows"
	"trylet/internal/trylet"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/icon.png
var appIcon []byte

var instanceID = "trylet.desktop"

func init() {
	application.RegisterEvent[trylet.RunEvent]("trylet:run")
	application.RegisterEvent[string]("trylet:git-changed")
	application.RegisterEvent[ClipboardCount]("trylet:clipboard-count")
}

type Desktop struct {
	mu            sync.Mutex
	app           *application.App
	ball          *application.WebviewWindow
	editor        *application.WebviewWindow
	store         *trylet.Store
	runner        *trylet.Runner
	git           *trylet.GitReader
	shortcut      string
	notice        string
	ready         bool
	showRequested bool
	moveTimer     *time.Timer
	ballExpanded  bool
	clipboardWake chan struct{}
	clipboardStop chan struct{}
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--trylet-lfs-clean" {
		if err := trylet.GitLFSClean(os.Stdin, os.Stdout); err != nil {
			log.Print(err)
			os.Exit(1)
		}
		return
	}
	executable, err := os.Executable()
	if err != nil {
		startupError(err)
		return
	}
	root := filepath.Dir(executable)
	if len(os.Args) > 1 && os.Args[1] == "--trylet-cleanup" {
		if err = runCleanupMode(root, os.Args[2:]); err != nil {
			log.Print(err)
			os.Exit(1)
		}
		return
	}
	runtimeDir := filepath.Join(root, runtimeFolder)
	store, err := trylet.NewStore(root, runtimeDir)
	if err != nil {
		startupError(err)
		return
	}
	desktop := &Desktop{store: store, clipboardWake: make(chan struct{}, 1), clipboardStop: make(chan struct{})}
	runner := trylet.NewRunner(runtimeDir, func(event trylet.RunEvent) {
		desktop.mu.Lock()
		editor := desktop.editor
		desktop.mu.Unlock()
		if editor != nil {
			editor.EmitEvent("trylet:run", event)
		}
	})
	desktop.runner = runner
	service := trylet.NewService(store, runner, desktop.applySettings)
	gitReader := trylet.NewGitReader(store, runtimeDir, func(id string) {
		desktop.mu.Lock()
		window := desktop.editor
		desktop.mu.Unlock()
		if window != nil {
			window.EmitEvent("trylet:git-changed", id)
		}
	})
	desktop.git = gitReader
	webAssets, err := fs.Sub(assets, "frontend/dist")
	if err != nil {
		startupError(err)
		return
	}
	app := application.New(application.Options{
		Name: "Trylet", Description: "随手写，立即运行。", Icon: appIcon, LogLevel: slog.LevelWarn,
		Windows: application.WindowsOptions{
			DisableQuitOnLastWindowClosed: true, WebviewUserDataPath: filepath.Join(runtimeDir, "webview2"),
			WndProcInterceptor: func(_ uintptr, msg uint32, _, _ uintptr) (uintptr, bool) {
				if msg == w32.WM_CLIPBOARDUPDATE {
					desktop.RefreshClipboard()
				}
				return 0, false
			},
		},
		Logger:         slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})),
		Services:       []application.Service{application.NewService(service), application.NewService(desktop), application.NewService(gitReader)},
		Assets:         application.AssetOptions{Handler: application.AssetFileServerFS(webAssets)},
		SingleInstance: &application.SingleInstanceOptions{UniqueID: instanceID, OnSecondInstanceLaunch: func(application.SecondInstanceData) { desktop.ShowEditor() }},
		OnShutdown: func() {
			close(desktop.clipboardStop)
			w32.RemoveClipboardFormatListener(w32.HWND(desktop.ball.NativeWindow()))
			gitReader.Close()
			if id := runner.ActiveID(); id != "" {
				if err := runner.Stop(id); err != nil {
					log.Printf("结束运行：%v", err)
				}
			}
			desktop.savePosition()
		},
	})
	owner, err := prepareRuntime(root)
	if err != nil {
		startupError(fmt.Errorf("无法在 EXE 同级准备运行目录，不会改用其他目录：%w", err))
		return
	}
	defer func() {
		if err := startCleanup(executable, owner); err != nil {
			w32.MessageBox(0, "未能启动临时目录清理，下次启动时会重试："+err.Error(), "Trylet", w32.MB_OK|w32.MB_ICONERROR)
		}
	}()
	if err = setRuntimeEnvironment(runtimeDir); err != nil {
		startupError(err)
		return
	}
	desktop.app = app
	desktop.ball = app.Window.NewWithOptions(application.WebviewWindowOptions{Name: "ball", Title: "Trylet", Width: 48, Height: 48, Frameless: true, AlwaysOnTop: true, DisableResize: true, URL: "/ball.html", BackgroundColour: application.NewRGB(244, 244, 246), Windows: application.WindowsWindow{HiddenOnTaskbar: true, DisableFramelessWindowDecorations: true}, DefaultContextMenuDisabled: true})
	desktop.ball.OnWindowEvent(events.Common.WindowRuntimeReady, func(*application.WindowEvent) {
		log.Print("悬浮球界面就绪")
		desktop.placeBall()
		desktop.shapeBall()
		desktop.ball.Show()
		application.InvokeSync(func() {
			if !w32.AddClipboardFormatListener(w32.HWND(desktop.ball.NativeWindow())) {
				log.Print("监听剪贴板失败")
			}
		})
		desktop.RefreshClipboard()
	})
	desktop.ball.OnWindowEvent(events.Common.WindowDidResize, func(*application.WindowEvent) { desktop.shapeBall() })
	desktop.ball.OnWindowEvent(events.Common.WindowDidMove, func(*application.WindowEvent) {
		desktop.mu.Lock()
		if desktop.moveTimer != nil {
			desktop.moveTimer.Stop()
		}
		desktop.moveTimer = time.AfterFunc(350*time.Millisecond, desktop.savePosition)
		desktop.mu.Unlock()
	})
	tray := app.SystemTray.New()
	tray.SetIcon(appIcon)
	tray.SetTooltip("Trylet · 随手写，立即运行")
	tray.OnClick(desktop.ToggleEditor)
	menu := app.NewMenu()
	menu.Add("打开编辑器").OnClick(func(*application.Context) { desktop.ShowEditor() })
	menu.Add("显示悬浮球").OnClick(func(*application.Context) { desktop.placeBall(); desktop.ball.Show() })
	menu.Add("隐藏悬浮球").OnClick(func(*application.Context) { desktop.ball.Hide() })
	menu.AddSeparator()
	menu.Add("退出 Trylet").OnClick(func(*application.Context) { desktop.requestQuit() })
	tray.SetMenu(menu)
	desktop.ball.OnWindowEvent(events.Common.WindowClosing, func(e *application.WindowEvent) { desktop.ball.Hide(); e.Cancel() })
	app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		log.Print("桌面应用启动")
		go desktop.watchClipboard()
		desktop.placeBall()
		desktop.ball.Show()
		if err := desktop.applySettings(store.Settings()); err != nil {
			desktop.mu.Lock()
			desktop.notice = err.Error()
			desktop.mu.Unlock()
			log.Printf("快捷键：%v", err)
		}
	})
	if len(os.Args) > 1 && os.Args[1] == "--editor" {
		desktop.showRequested = true
		desktop.createEditor()
	}
	log.Print("进入应用事件循环")
	if err = app.Run(); err != nil {
		startupError(err)
	}
}

func startupError(err error) {
	log.Print(err)
	w32.MessageBox(0, "Trylet 启动失败："+err.Error(), "Trylet", w32.MB_OK|w32.MB_ICONERROR)
}

func (d *Desktop) createEditor() *application.WebviewWindow {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.editor != nil {
		return d.editor
	}
	d.editor = d.app.Window.NewWithOptions(application.WebviewWindowOptions{Name: "editor", Title: "Trylet", Width: 720, Height: 560, MinWidth: 500, MinHeight: 380, Frameless: true, Hidden: true, BackgroundColour: application.NewRGB(244, 244, 246), DefaultContextMenuDisabled: true})
	d.editor.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) { e.Cancel(); d.requestHide() })
	d.editor.OnWindowEvent(events.Common.WindowRuntimeReady, func(*application.WindowEvent) {
		d.mu.Lock()
		show := d.showRequested
		d.mu.Unlock()
		if show {
			d.ShowEditor()
		}
	})
	return d.editor
}

func (d *Desktop) ShowEditor() {
	d.git.SetVisible(true)
	d.mu.Lock()
	d.showRequested = true
	d.mu.Unlock()
	window := d.createEditor()
	if !window.IsVisible() {
		screen, err := d.ball.GetScreen()
		if err != nil || screen == nil {
			screen = d.app.Screen.GetPrimary()
		}
		if screen != nil {
			x, y := d.ball.Position()
			w, h := window.Size()
			area := screen.WorkArea
			x = max(area.X+8, min(x-w-12, area.X+area.Width-w-8))
			y = max(area.Y+8, min(y-32, area.Y+area.Height-h-8))
			window.SetPosition(x, y)
		}
	}
	window.Show()
	window.Focus()
}
func (d *Desktop) ToggleEditor() {
	d.mu.Lock()
	window := d.editor
	d.mu.Unlock()
	if window != nil && window.IsVisible() {
		d.requestHide()
	} else {
		d.ShowEditor()
	}
}
func (d *Desktop) SyncBallDPI() {
	if d.ball != nil {
		d.shapeBall()
	}
}
func (d *Desktop) requestHide() {
	d.mu.Lock()
	window, ready := d.editor, d.ready
	d.mu.Unlock()
	if window == nil {
		return
	}
	if ready {
		window.EmitEvent("trylet:hide-requested")
	} else {
		window.Hide()
	}
}
func (d *Desktop) HideEditor() {
	d.git.SetVisible(false)
	d.mu.Lock()
	d.showRequested = false
	window := d.editor
	d.mu.Unlock()
	if window != nil {
		window.Hide()
	}
}
func (d *Desktop) EditorReady() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.ready = true
	return d.notice
}
func (d *Desktop) SetPinned(value bool) {
	d.mu.Lock()
	window := d.editor
	d.mu.Unlock()
	if window != nil {
		window.SetAlwaysOnTop(value)
	}
}
func (d *Desktop) Quit() error {
	if id := d.runner.ActiveID(); id != "" {
		if err := d.runner.Stop(id); err != nil {
			return err
		}
	}
	d.app.Quit()
	return nil
}
func (d *Desktop) requestQuit() {
	d.mu.Lock()
	ready := d.ready
	d.mu.Unlock()
	if !ready {
		_ = d.Quit()
		return
	}
	d.ShowEditor()
	d.mu.Lock()
	window := d.editor
	d.mu.Unlock()
	window.EmitEvent("trylet:quit-requested")
}
func (d *Desktop) ChooseGitRepository() (string, error) {
	return d.app.Dialog.OpenFile().CanChooseDirectories(true).CanChooseFiles(false).CanCreateDirectories(false).SetTitle("打开 Git 仓库").AttachToWindow(d.editor).PromptForSingleSelection()
}
func (d *Desktop) ChooseSkillsDirectory() (string, error) {
	return d.app.Dialog.OpenFile().CanChooseDirectories(true).CanChooseFiles(false).CanCreateDirectories(false).SetTitle("选择项目或 Skills 目录").AttachToWindow(d.editor).PromptForSingleSelection()
}
func (d *Desktop) CopyText(text string) error {
	if !d.app.Clipboard.SetText(text) {
		return fmt.Errorf("复制失败，请重试")
	}
	return nil
}

type ClipboardCount struct {
	HasText       bool   `json:"hasText"`
	WithSpaces    int    `json:"withSpaces"`
	WithoutSpaces int    `json:"withoutSpaces"`
	Error         string `json:"error,omitempty"`
}

func countCharacters(text string) ClipboardCount {
	count := ClipboardCount{HasText: true}
	for _, character := range text {
		// 按 Word 的纯文本统计口径，段落、分页和零宽空格不计为字符。
		switch character {
		case '\r', '\n', '\f', '\u200b', '\ufeff':
			continue
		}
		count.WithSpaces++
		switch character {
		case ' ', '\t', '\v', '\u00a0', '\u2005', '\u3000':
			continue
		}
		count.WithoutSpaces++
	}
	return count
}

func (d *Desktop) RefreshClipboard() {
	select {
	case d.clipboardWake <- struct{}{}:
	default:
	}
}

func readClipboardCount() (ClipboardCount, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	for attempt := 0; !w32.OpenClipboard(0); attempt++ {
		if attempt == 99 {
			return ClipboardCount{}, fmt.Errorf("剪贴板被其他程序占用")
		}
		time.Sleep(10 * time.Millisecond)
	}
	defer w32.CloseClipboard()
	if !w32.IsClipboardFormatAvailable(w32.CF_UNICODETEXT) {
		return ClipboardCount{}, nil
	}
	handle := w32.GetClipboardData(w32.CF_UNICODETEXT)
	kernel := windows.NewLazySystemDLL("kernel32.dll")
	size, _, err := kernel.NewProc("GlobalSize").Call(uintptr(handle))
	if size < 2 {
		return ClipboardCount{}, fmt.Errorf("读取剪贴板文本：%w", err)
	}
	pointer := w32.GlobalLock(w32.HGLOBAL(handle))
	// GlobalUnlock 返回 0 也可能表示成功；按实际分配长度读取，避免大段文本被截断。
	defer w32.GlobalUnlock(w32.HGLOBAL(handle))
	text := windows.UTF16ToString(unsafe.Slice((*uint16)(pointer), int(size/2)))
	return countCharacters(text), nil
}

func (d *Desktop) watchClipboard() {
	for {
		select {
		case <-d.clipboardStop:
			return
		case <-d.clipboardWake:
			count, err := readClipboardCount()
			if err != nil {
				count.Error = "读取剪贴板失败，请重新复制"
				log.Printf("统计剪贴板字符：%v", err)
			}
			d.mu.Lock()
			resize := d.ballExpanded != count.HasText
			d.ballExpanded = count.HasText
			d.mu.Unlock()
			if resize {
				d.shapeBall()
			}
			d.ball.EmitEvent("trylet:clipboard-count", count)
		}
	}
}

func (d *Desktop) OpenURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host != "github.com" {
		return fmt.Errorf("只能打开 GitHub HTTPS 链接")
	}
	return d.app.Browser.OpenURL(raw)
}

func (d *Desktop) Appearance() string { return d.store.Settings().Theme }

func (d *Desktop) applySettings(v trylet.Settings) (err error) {
	defer func() {
		if err == nil && d.ball != nil {
			d.ball.EmitEvent("trylet:appearance", v.Theme)
		}
	}()
	d.mu.Lock()
	old := d.shortcut
	d.mu.Unlock()
	if old == v.Shortcut {
		return nil
	}
	if v.Shortcut != "" {
		if err := d.app.GlobalShortcut.Register(v.Shortcut, d.ToggleEditor); err != nil {
			return fmt.Errorf("快捷键未注册，可能已被占用：%w", err)
		}
	}
	if old != "" {
		if err := d.app.GlobalShortcut.Unregister(old); err != nil {
			if v.Shortcut != "" {
				_ = d.app.GlobalShortcut.Unregister(v.Shortcut)
			}
			return err
		}
	}
	d.mu.Lock()
	d.shortcut = v.Shortcut
	d.notice = ""
	d.mu.Unlock()
	return nil
}

func (d *Desktop) placeBall() {
	v := d.store.Settings()
	screens := d.app.Screen.GetAll()
	x, y := v.BallX, v.BallY
	visible := false
	if v.BallPlaced {
		for _, screen := range screens {
			a := screen.WorkArea
			if x >= a.X && y >= a.Y && x+48 <= a.X+a.Width && y+48 <= a.Y+a.Height {
				visible = true
				break
			}
		}
	}
	if !visible {
		if screen := d.app.Screen.GetPrimary(); screen != nil {
			a := screen.WorkArea
			x = a.X + a.Width - 72
			y = a.Y + a.Height/2
		}
	}
	width, _ := d.ball.Size()
	d.ball.SetPosition(x-width+48, y)
	d.shapeBall()
}

func (d *Desktop) savePosition() {
	if d.ball == nil {
		return
	}
	x, y := d.ball.Position()
	width, _ := d.ball.Size()
	x += width - 48
	if err := d.store.SaveBallPosition(x, y); err != nil {
		log.Printf("保存悬浮球位置：%v", err)
	}
}

func (d *Desktop) shapeBall() {
	application.InvokeSync(func() {
		hwnd := uintptr(d.ball.NativeWindow())
		if hwnd == 0 {
			return
		}
		bounds := d.ball.PhysicalBounds()
		center := application.Point{X: bounds.X + bounds.Width - bounds.Height/2, Y: bounds.Y + bounds.Height/2}
		screen := application.ScreenNearestPhysicalPoint(center)
		if screen == nil {
			log.Print("读取悬浮球所在屏幕失败")
			return
		}
		d.mu.Lock()
		width := 48
		if d.ballExpanded {
			width = 192
		}
		d.mu.Unlock()
		// 按所在屏幕校正 DIP，展开时保持右侧图标的位置。
		size := int(math.Round(48 * float64(screen.ScaleFactor)))
		width = int(math.Round(float64(width) * float64(screen.ScaleFactor)))
		area := screen.PhysicalWorkArea
		x := max(area.X, min(center.X-width+size/2, area.X+area.Width-width))
		y := max(area.Y, min(center.Y-size/2, area.Y+area.Height-size))
		if bounds.X != x || bounds.Y != y || bounds.Width != width || bounds.Height != size {
			bounds.X, bounds.Y, bounds.Width, bounds.Height = x, y, width, size
			d.ball.SetPhysicalBounds(bounds)
		}
		region, _, err := windows.NewLazySystemDLL("gdi32.dll").NewProc("CreateRoundRectRgn").Call(0, 0, uintptr(width+1), uintptr(size+1), uintptr(size), uintptr(size))
		if region == 0 {
			log.Printf("创建悬浮球形状：%v", err)
			return
		}
		ok, _, err := windows.NewLazySystemDLL("user32.dll").NewProc("SetWindowRgn").Call(hwnd, region, 1)
		if ok == 0 {
			_, _, _ = windows.NewLazySystemDLL("gdi32.dll").NewProc("DeleteObject").Call(region)
			log.Printf("设置悬浮球形状：%v", err)
		}
	})
}

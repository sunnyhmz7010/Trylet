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
	"sync"
	"time"

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
	desktop := &Desktop{store: store}
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
		Windows:        application.WindowsOptions{DisableQuitOnLastWindowClosed: true, WebviewUserDataPath: filepath.Join(runtimeDir, "webview2")},
		Logger:         slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})),
		Services:       []application.Service{application.NewService(service), application.NewService(desktop), application.NewService(gitReader)},
		Assets:         application.AssetOptions{Handler: application.AssetFileServerFS(webAssets)},
		SingleInstance: &application.SingleInstanceOptions{UniqueID: instanceID, OnSecondInstanceLaunch: func(application.SecondInstanceData) { desktop.ShowEditor() }},
		OnShutdown: func() {
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
func (d *Desktop) CopyText(text string) error {
	if !d.app.Clipboard.SetText(text) {
		return fmt.Errorf("复制失败，请重试")
	}
	return nil
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
	d.ball.SetPosition(x, y)
}

func (d *Desktop) savePosition() {
	if d.ball == nil {
		return
	}
	x, y := d.ball.Position()
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
		center := application.Point{X: bounds.X + bounds.Width/2, Y: bounds.Y + bounds.Height/2}
		screen := application.ScreenNearestPhysicalPoint(center)
		if screen == nil {
			log.Print("读取悬浮球所在屏幕失败")
			return
		}
		// 小窗口的原生 DPI 通知可能滞后，按所在屏幕缩放直接校正 48 DIP，避免坐标换算的取整偏差。
		size := int(math.Round(48 * float64(screen.ScaleFactor)))
		if bounds.Width != size || bounds.Height != size {
			// 保持选屏锚点不变，跨屏边界处不会因尺寸变化反复切换目标屏幕。
			bounds.X, bounds.Y = center.X-size/2, center.Y-size/2
			bounds.Width, bounds.Height = size, size
			d.ball.SetPhysicalBounds(bounds)
		}
		region, _, err := windows.NewLazySystemDLL("gdi32.dll").NewProc("CreateEllipticRgn").Call(0, 0, uintptr(size), uintptr(size))
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

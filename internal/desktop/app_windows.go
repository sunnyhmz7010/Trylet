package desktop

import (
	"fmt"
	"io/fs"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/w32"

	appservice "trylet/internal/app"
	gitrepo "trylet/internal/git"
	githubapi "trylet/internal/github"
	"trylet/internal/portable"
	execution "trylet/internal/runner"
	"trylet/internal/skills"
	storage "trylet/internal/store"
	"trylet/internal/textstats"
)

var instanceID = "trylet.desktop"

func init() {
	application.RegisterEvent[execution.RunEvent]("trylet:run")
	application.RegisterEvent[skills.SkillDownloadProgress]("trylet:skill-download")
	application.RegisterEvent[string]("trylet:git-changed")
	application.RegisterEvent[textstats.SelectionCount]("trylet:selection-count")
	application.RegisterEvent[bool]("trylet:window-mode")
	application.RegisterEvent[storage.Settings]("trylet:settings")
}

// Run 启动桌面应用，并处理同一可执行文件的辅助模式。
func Run(assets fs.FS, appIcon []byte) {
	if len(os.Args) > 1 && os.Args[1] == "--trylet-lfs-clean" {
		if err := gitrepo.GitLFSClean(os.Stdin, os.Stdout); err != nil {
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
		if err = portable.RunCleanupMode(root, os.Args[2:]); err != nil {
			log.Print(err)
			os.Exit(1)
		}
		return
	}
	runtimeDir := filepath.Join(root, portable.RuntimeFolder)
	store, err := storage.NewStore(root, runtimeDir)
	if err != nil && storage.CanRestoreStoreBackup(root) {
		message := "数据文件已损坏，是否从上一次自动备份恢复？\n\n恢复前会另存保留当前文件。备份可能不包含最后一次编辑。"
		if w32.MessageBox(0, message, "恢复 Trylet 数据", w32.MB_YESNO|w32.MB_ICONWARNING|w32.MB_DEFBUTTON2) != w32.IDYES {
			return
		}
		if _, restoreErr := storage.RestoreStoreBackup(root); restoreErr != nil {
			startupError(restoreErr)
			return
		}
		store, err = storage.NewStore(root, runtimeDir)
	}
	if err != nil {
		startupError(err)
		return
	}
	desktop := &Desktop{store: store, selectionWake: make(chan struct{}, 1), selectionStop: make(chan struct{})}
	runner := execution.NewRunner(runtimeDir, func(event execution.RunEvent) {
		desktop.mu.Lock()
		editor := desktop.editor
		desktop.mu.Unlock()
		if editor != nil {
			editor.EmitEvent("trylet:run", event)
		}
	})
	desktop.runner = runner
	publisher := githubapi.NewPublisher(store.ReadToken, "https://api.github.com")
	service := appservice.NewService(store, runner, publisher, desktop.applySettings)
	skillService := skills.NewService(store, publisher, func(progress skills.SkillDownloadProgress) {
		desktop.mu.Lock()
		window := desktop.editor
		desktop.mu.Unlock()
		if window != nil {
			window.EmitEvent("trylet:skill-download", progress)
		}
	})
	gitReader := gitrepo.NewGitReader(store, runtimeDir, func(id string) {
		desktop.mu.Lock()
		window := desktop.editor
		desktop.mu.Unlock()
		if window != nil {
			window.EmitEvent("trylet:git-changed", id)
		}
	})
	desktop.git = gitReader
	webAssets, err := fs.Sub(assets, "dist")
	if err != nil {
		startupError(err)
		return
	}
	app := application.New(application.Options{
		Name: "Trylet", Description: "随手写，立即运行。", Icon: appIcon, LogLevel: slog.LevelWarn,
		Windows: application.WindowsOptions{
			DisableQuitOnLastWindowClosed: true, WebviewUserDataPath: filepath.Join(runtimeDir, "webview2"),
		},
		Logger:         slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})),
		Services:       []application.Service{application.NewService(service), application.NewService(desktop), application.NewService(gitReader), application.NewService(skillService)},
		Assets:         application.AssetOptions{Handler: application.AssetFileServerFS(webAssets)},
		SingleInstance: &application.SingleInstanceOptions{UniqueID: instanceID, OnSecondInstanceLaunch: func(application.SecondInstanceData) { desktop.ShowEditor() }},
		OnShutdown: func() {
			close(desktop.selectionStop)
			gitReader.Close()
			if id := runner.ActiveID(); id != "" {
				if err := runner.Stop(id); err != nil {
					log.Printf("结束运行：%v", err)
				}
			}
			desktop.savePosition()
		},
	})
	owner, err := portable.PrepareRuntime(root)
	if err != nil {
		startupError(fmt.Errorf("无法在 EXE 同级准备运行目录，不会改用其他目录：%w", err))
		return
	}
	defer func() {
		if err := portable.StartCleanup(executable, owner); err != nil {
			w32.MessageBox(0, "未能启动临时目录清理，下次启动时会重试："+err.Error(), "Trylet", w32.MB_OK|w32.MB_ICONERROR)
		}
	}()
	if err = portable.SetRuntimeEnvironment(runtimeDir); err != nil {
		startupError(err)
		return
	}
	desktop.app = app
	desktop.ball = app.Window.NewWithOptions(application.WebviewWindowOptions{Name: "ball", Title: "Trylet 悬浮窗", Width: 48, Height: 48, Hidden: true, Frameless: true, AlwaysOnTop: true, DisableResize: true, URL: "/ball.html", BackgroundColour: application.NewRGB(244, 244, 246), Windows: application.WindowsWindow{HiddenOnTaskbar: true, DisableFramelessWindowDecorations: true}, DefaultContextMenuDisabled: true})
	desktop.ball.OnWindowEvent(events.Common.WindowRuntimeReady, func(*application.WindowEvent) {
		log.Print("悬浮球界面就绪")
		desktop.mu.Lock()
		desktop.ballReady = true
		desktop.mu.Unlock()
		desktop.placeBall()
		desktop.shapeBall()
		desktop.syncBall(store.Settings())
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
	tray.OnClick(desktop.ShowEditor)
	menu := app.NewMenu()
	menu.Add("打开编辑器").OnClick(func(*application.Context) { desktop.ShowEditor() })
	ballVisibility := menu.Add("关闭悬浮窗").OnClick(func(*application.Context) {
		settings := store.Settings()
		settings.BallEnabled = !settings.BallEnabled
		if err := service.SaveSettings(settings); err != nil {
			w32.MessageBox(0, "保存悬浮窗设置失败："+err.Error(), "Trylet", w32.MB_OK|w32.MB_ICONERROR)
			return
		}
		desktop.mu.Lock()
		window := desktop.editor
		desktop.mu.Unlock()
		if window != nil {
			window.EmitEvent("trylet:settings", store.Settings())
		}
	})
	menu.AddSeparator()
	menu.Add("退出 Trylet").OnClick(func(*application.Context) { desktop.requestQuit() })
	tray.SetMenu(menu)
	tray.OnRightClick(func() {
		if store.Settings().BallEnabled {
			ballVisibility.SetLabel("关闭悬浮窗")
		} else {
			ballVisibility.SetLabel("开启悬浮窗")
		}
		tray.OpenMenu()
	})
	desktop.ball.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) { e.Cancel(); desktop.ball.Hide() })
	app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		log.Print("桌面应用启动")
		go desktop.watchSelection()
		if err := desktop.applySettings(store.Settings()); err != nil {
			desktop.mu.Lock()
			desktop.notice = err.Error()
			desktop.mu.Unlock()
			log.Printf("快捷键：%v", err)
		}
	})
	desktop.ShowEditor()
	log.Print("进入应用事件循环")
	if err = app.Run(); err != nil {
		startupError(err)
	}
}

func startupError(err error) {
	log.Print(err)
	w32.MessageBox(0, "Trylet 启动失败："+err.Error(), "Trylet", w32.MB_OK|w32.MB_ICONERROR)
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

func (d *Desktop) applySettings(v storage.Settings) (err error) {
	defer func() {
		if err == nil && d.ball != nil {
			d.ball.EmitEvent("trylet:appearance", v.Theme)
			d.syncBall(v)
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

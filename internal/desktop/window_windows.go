package desktop

import (
	"fmt"
	"net/url"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	storage "trylet/internal/store"
)

func (d *Desktop) createEditor() *application.WebviewWindow {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.editor != nil {
		return d.editor
	}
	d.editor = d.app.Window.NewWithOptions(application.WebviewWindowOptions{Name: "editor", Title: "Trylet", Width: 720, Height: 560, MinWidth: 500, MinHeight: 380, Frameless: true, Hidden: true, InitialPosition: application.WindowCentered, BackgroundColour: application.NewRGB(244, 244, 246), DefaultContextMenuDisabled: true})
	d.editor.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) { e.Cancel(); d.CloseEditor() })
	for _, event := range []events.WindowEventType{events.Common.WindowShow, events.Common.WindowHide, events.Common.WindowMinimise, events.Common.WindowUnMinimise, events.Common.WindowMaximise, events.Common.WindowUnMaximise, events.Common.WindowRestore} {
		d.editor.OnWindowEvent(event, func(*application.WindowEvent) {
			d.syncBall(d.store.Settings())
			d.editor.EmitEvent("trylet:window-mode", d.editor.IsMaximised())
		})
	}
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
	window.Show()
	window.UnMinimise()
	window.Focus()
	d.syncBall(d.store.Settings())
}

func (d *Desktop) ToggleEditor() {
	d.mu.Lock()
	window := d.editor
	d.mu.Unlock()
	if window != nil && window.IsVisible() && !window.IsMinimised() {
		d.requestHide()
	} else {
		d.ShowEditor()
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
		d.HideEditor()
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
	d.syncBall(d.store.Settings())
}

func (d *Desktop) CloseEditor() {
	settings := d.store.Settings()
	if !settings.RememberCloseAction {
		d.ShowEditor()
		d.mu.Lock()
		if !d.ready {
			d.closeRequested = true
			d.mu.Unlock()
			return
		}
		window := d.editor
		d.mu.Unlock()
		window.EmitEvent("trylet:close-requested")
		return
	}
	if settings.CloseAction == "quit" {
		d.requestQuit()
	} else {
		d.requestHide()
	}
}

func (d *Desktop) RememberCloseAction(action string) (storage.Settings, error) {
	if err := d.store.RememberCloseAction(action); err != nil {
		return storage.Settings{}, err
	}
	return d.store.Settings(), nil
}

func (d *Desktop) MinimiseEditor() {
	d.mu.Lock()
	window := d.editor
	d.mu.Unlock()
	if window != nil {
		window.Minimise()
		d.syncBall(d.store.Settings())
	}
}

func (d *Desktop) ToggleWindowMode() bool {
	d.mu.Lock()
	window := d.editor
	d.mu.Unlock()
	if window == nil {
		return false
	}
	if window.IsMaximised() {
		window.UnMaximise()
	} else {
		window.Maximise()
	}
	return window.IsMaximised()
}

func (d *Desktop) EditorReady() string {
	d.mu.Lock()
	d.ready = true
	closeRequested := d.closeRequested
	d.closeRequested = false
	window, notice := d.editor, d.notice
	d.mu.Unlock()
	if closeRequested {
		window.EmitEvent("trylet:close-requested")
	}
	return notice
}

func (d *Desktop) SetPinned(value bool) {
	d.mu.Lock()
	window := d.editor
	d.mu.Unlock()
	if window != nil {
		window.SetAlwaysOnTop(value)
	}
}

func (d *Desktop) ChooseGitRepository() (string, error) {
	return d.app.Dialog.OpenFile().CanChooseDirectories(true).CanChooseFiles(false).CanCreateDirectories(false).SetTitle("打开 Git 仓库").AttachToWindow(d.editor).PromptForSingleSelection()
}

func (d *Desktop) ChooseSkillsDirectory() (string, error) {
	return d.app.Dialog.OpenFile().CanChooseDirectories(true).CanChooseFiles(false).CanCreateDirectories(false).SetTitle("选择项目或 Skills 目录").AttachToWindow(d.editor).PromptForSingleSelection()
}

func (d *Desktop) ExportDraft(id, language string) (string, error) {
	extension := map[string]string{"c": ".c", "cpp": ".cpp", "python": ".py"}[language]
	if extension == "" {
		extension = ".txt"
	}
	dialog := d.app.Dialog.SaveFile()
	dialog.SetOptions(&application.SaveFileDialogOptions{Title: "导出当前草稿", Filename: "main" + extension, CanCreateDirectories: true})
	path, err := dialog.AddFilter("源码文件", "*"+extension).AttachToWindow(d.editor).PromptForSingleSelection()
	if err != nil || path == "" {
		return "", err
	}
	if err := d.store.ExportDraft(id, path); err != nil {
		return "", fmt.Errorf("导出草稿失败：%w", err)
	}
	return path, nil
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

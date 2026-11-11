package desktop

import (
	"log"
	"math"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/w32"
	"golang.org/x/sys/windows"

	storage "trylet/internal/store"
)

func (d *Desktop) SyncBallDPI() {
	if d.ball != nil {
		d.shapeBall()
	}
}

func (d *Desktop) SetBallWidth(width int) {
	d.mu.Lock()
	d.ballWidth = max(48, width)
	d.mu.Unlock()
	d.shapeBall()
}

func (d *Desktop) syncBall(v storage.Settings) {
	d.mu.Lock()
	ball, window, ready := d.ball, d.editor, d.ballReady
	d.mu.Unlock()
	if ball == nil || !ready {
		return
	}
	visible := window != nil && window.IsVisible() && !window.IsMinimised()
	if v.BallEnabled && (!v.BallFollowWindow || !visible) {
		if !ball.IsVisible() {
			application.InvokeSync(func() {
				// 仅显示时禁止激活，随后恢复样式，保留悬浮窗的点击与拖动。
				hwnd := w32.HWND(ball.NativeWindow())
				style := w32.GetWindowLongPtr(hwnd, w32.GWL_EXSTYLE)
				w32.SetWindowLongPtr(hwnd, w32.GWL_EXSTYLE, style|w32.WS_EX_NOACTIVATE)
				ball.Show()
				w32.SetWindowLongPtr(hwnd, w32.GWL_EXSTYLE, style)
			})
			d.RefreshSelection()
		}
	} else if ball.IsVisible() {
		ball.Hide()
	}
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
		width := max(48, d.ballWidth)
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

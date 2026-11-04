package main

import (
	"errors"
	"fmt"
	"log"
	"runtime"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"

	"trylet/internal/trylet"

	"github.com/wailsapp/wails/v3/pkg/w32"
	"golang.org/x/sys/windows"
)

// COM 虚表下标对应 Windows SDK 的 UIAutomationClient.h，兼容 x64 / ARM64。
type selectionObject struct {
	vtable *[64]uintptr
}

//go:uintptrescapes
func (o *selectionObject) call(method int, args ...uintptr) error {
	args = append([]uintptr{uintptr(unsafe.Pointer(o))}, args...)
	status, _, _ := syscall.SyscallN(o.vtable[method], args...)
	if int32(status) < 0 {
		return syscall.Errno(uint32(status))
	}
	return nil
}

func (o *selectionObject) release() {
	syscall.SyscallN(o.vtable[2], uintptr(unsafe.Pointer(o)))
}

func newSelectionAutomation() (*selectionObject, error) {
	classID := windows.GUID{Data1: 0xe22ad333, Data2: 0xb25f, Data3: 0x460c, Data4: [8]byte{0x83, 0xd0, 0x05, 0x81, 0x10, 0x73, 0x95, 0xc9}}
	interfaceID := windows.GUID{Data1: 0x34723aff, Data2: 0x0c9d, Data3: 0x49d0, Data4: [8]byte{0x98, 0x96, 0x7a, 0xb5, 0x2d, 0xf8, 0xcd, 0x8a}}
	var automation *selectionObject
	status, _, _ := windows.NewLazySystemDLL("ole32.dll").NewProc("CoCreateInstance").Call(
		uintptr(unsafe.Pointer(&classID)), 0, windows.CLSCTX_INPROC_SERVER,
		uintptr(unsafe.Pointer(&interfaceID)), uintptr(unsafe.Pointer(&automation)),
	)
	if int32(status) < 0 {
		return nil, fmt.Errorf("创建 UI Automation：HRESULT 0x%08X", uint32(status))
	}
	// IUIAutomation2：读取选区不得抢焦点，无响应应用不得长期阻塞更新。
	for _, setting := range []struct {
		method int
		value  uintptr
	}{{59, 0}, {61, 250}, {63, 250}} {
		if err := automation.call(setting.method, setting.value); err != nil {
			automation.release()
			return nil, fmt.Errorf("设置选区读取策略：%w", err)
		}
	}
	return automation, nil
}

func readSelectionCount(automation, walker *selectionObject) (trylet.SelectionCount, error) {
	foreground := w32.GetForegroundWindow()
	if foreground == 0 {
		return trylet.SelectionCount{}, nil
	}
	var element *selectionObject
	if err := automation.call(8, uintptr(unsafe.Pointer(&element))); err != nil {
		return trylet.SelectionCount{}, fmt.Errorf("读取焦点控件：%w", err)
	}
	defer func() {
		if element != nil {
			element.release()
		}
	}()
	textPatternID := windows.GUID{Data1: 0x32eba289, Data2: 0x3583, Data3: 0x42c9, Data4: [8]byte{0x9c, 0x59, 0x3b, 0x6d, 0x9a, 0x1e, 0x9b, 0x6a}}
	for element != nil {
		var password int32
		if err := element.call(35, uintptr(unsafe.Pointer(&password))); err != nil {
			return trylet.SelectionCount{}, fmt.Errorf("读取控件类型：%w", err)
		}
		if password != 0 {
			return trylet.SelectionCount{}, nil
		}
		var pattern *selectionObject
		err := element.call(14, 10014, uintptr(unsafe.Pointer(&textPatternID)), uintptr(unsafe.Pointer(&pattern)))
		if err != nil && !errors.Is(err, syscall.Errno(0x80040204)) && !errors.Is(err, syscall.Errno(0x80004002)) {
			return trylet.SelectionCount{}, fmt.Errorf("读取文字选区接口：%w", err)
		}
		if pattern != nil {
			defer pattern.release()
			count, err := countSelectedRanges(pattern)
			if foreground != w32.GetForegroundWindow() {
				return trylet.SelectionCount{}, nil
			}
			return count, err
		}
		// 浏览器等应用的焦点可能位于文字节点，向上找到所属文字容器。
		var parent *selectionObject
		if err := walker.call(3, uintptr(unsafe.Pointer(element)), uintptr(unsafe.Pointer(&parent))); err != nil {
			return trylet.SelectionCount{}, fmt.Errorf("读取文字容器：%w", err)
		}
		element.release()
		element = parent
	}
	return trylet.SelectionCount{}, nil
}

func countSelectedRanges(pattern *selectionObject) (trylet.SelectionCount, error) {
	var ranges *selectionObject
	if err := pattern.call(5, uintptr(unsafe.Pointer(&ranges))); err != nil {
		return trylet.SelectionCount{}, fmt.Errorf("读取选区：%w", err)
	}
	if ranges == nil {
		return trylet.SelectionCount{}, nil
	}
	defer ranges.release()
	var length int32
	if err := ranges.call(3, uintptr(unsafe.Pointer(&length))); err != nil {
		return trylet.SelectionCount{}, fmt.Errorf("读取选区数量：%w", err)
	}
	oleaut := windows.NewLazySystemDLL("oleaut32.dll")
	count := trylet.SelectionCount{}
	for index := int32(0); index < length; index++ {
		var selected *selectionObject
		if err := ranges.call(4, uintptr(index), uintptr(unsafe.Pointer(&selected))); err != nil {
			return trylet.SelectionCount{}, fmt.Errorf("读取选区范围：%w", err)
		}
		var text *uint16
		// -1 表示完整选区；退化为光标的空范围返回空文本，自动触发收起。
		err := selected.call(12, ^uintptr(0), uintptr(unsafe.Pointer(&text)))
		selected.release()
		if err != nil {
			return trylet.SelectionCount{}, fmt.Errorf("读取选中文字：%w", err)
		}
		if text != nil {
			textLength, _, _ := oleaut.NewProc("SysStringLen").Call(uintptr(unsafe.Pointer(text)))
			part := trylet.CountCharacters(string(utf16.Decode(unsafe.Slice(text, int(textLength)))))
			oleaut.NewProc("SysFreeString").Call(uintptr(unsafe.Pointer(text)))
			count.HasText = count.HasText || part.HasText
			count.WithSpaces += part.WithSpaces
			count.WithoutSpaces += part.WithoutSpaces
		}
	}
	return count, nil
}

func (d *Desktop) RefreshSelection() {
	select {
	case d.selectionWake <- struct{}{}:
	default:
	}
}

func (d *Desktop) watchSelection() {
	// UI Automation 固定在独立 MTA 线程，避免与 WebView 的界面线程互相等待。
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := windows.CoInitializeEx(0, windows.COINIT_MULTITHREADED); err != nil && !errors.Is(err, syscall.Errno(windows.S_FALSE)) {
		log.Printf("初始化选区统计：%v", err)
		return
	}
	defer windows.CoUninitialize()
	automation, err := newSelectionAutomation()
	if err != nil {
		log.Printf("初始化选区统计：%v", err)
		return
	}
	defer automation.release()
	var walker *selectionObject
	if err := automation.call(14, uintptr(unsafe.Pointer(&walker))); err != nil {
		log.Printf("初始化选区控件遍历：%v", err)
		return
	}
	defer walker.release()
	ticker := time.NewTicker(150 * time.Millisecond)
	defer ticker.Stop()
	previous := trylet.SelectionCount{}
	lastError := ""
	for {
		force := false
		select {
		case <-d.selectionStop:
			return
		case <-d.selectionWake:
			force = true
		case <-ticker.C:
		}
		count := trylet.SelectionCount{}
		var readErr error
		if d.ball.IsVisible() {
			count, readErr = readSelectionCount(automation, walker)
		}
		message := ""
		if readErr != nil {
			message = readErr.Error()
			count.Error = "暂时无法读取当前文字选区"
			if message != lastError {
				log.Printf("统计选中文字：%v", readErr)
			}
		}
		lastError = message
		if !force && count == previous {
			continue
		}
		previous = count
		d.ball.EmitEvent("trylet:selection-count", count)
	}
}

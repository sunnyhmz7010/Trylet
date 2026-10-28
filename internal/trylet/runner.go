package trylet

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unsafe"

	pty "github.com/aymanbagabas/go-pty"
	"golang.org/x/sys/windows"
)

type runSession struct {
	outputMu    sync.Mutex
	outputBytes int
	truncated   bool
	id          string
	job         windows.Handle
	cancel      context.CancelFunc
	done        chan struct{}
	terminal    pty.Pty
	phase       string
}
type Runner struct {
	mu     sync.Mutex
	root   string
	active *runSession
	emit   func(RunEvent)
}

func NewRunner(root string, emit func(RunEvent)) *Runner { return &Runner{root: root, emit: emit} }

func createJob() (windows.Handle, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return 0, err
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	_, err = windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)))
	if err != nil {
		_ = windows.CloseHandle(job)
		return 0, err
	}
	return job, nil
}

func assignJob(job windows.Handle, pid int) error {
	h, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	return windows.AssignProcessToJobObject(job, h)
}

func (r *Runner) ActiveID() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.active == nil {
		return ""
	}
	return r.active.id
}

func (r *Runner) Start(req RunRequest, runtime RuntimeCandidate) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.active != nil {
		return "", errors.New("请先停止当前运行")
	}
	if !validID(req.ID) || !validID(req.DraftID) || extension(req.Language) == "" || len(req.Code) > 1024*1024 {
		return "", errors.New("运行参数无效")
	}
	job, err := createJob()
	if err != nil {
		return "", fmt.Errorf("创建运行进程组失败：%w", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	session := &runSession{id: req.ID, job: job, cancel: cancel, done: make(chan struct{}), phase: "starting"}
	r.active = session
	go r.execute(ctx, session, req, runtime)
	return session.id, nil
}

func (r *Runner) phase(session *runSession, phase string) {
	r.mu.Lock()
	session.phase = phase
	r.mu.Unlock()
	r.emit(RunEvent{ID: session.id, Kind: "state", Phase: phase})
}

func (r *Runner) output(id string, b []byte) {
	if len(b) == 0 {
		return
	}
	r.mu.Lock()
	session := r.active
	r.mu.Unlock()
	if session == nil || session.id != id {
		return
	}
	session.outputMu.Lock()
	defer session.outputMu.Unlock()
	if session.truncated {
		return
	}
	if session.outputBytes+len(b) > 1024*1024 {
		session.truncated = true
		b = []byte("\x18\x1b[0m\r\n[输出已达 1 MiB，后续输出已截断；程序仍可停止。]\r\n")
	} else {
		session.outputBytes += len(b)
	}
	r.emit(RunEvent{ID: id, Kind: "output", Data: base64.StdEncoding.EncodeToString(b)})
}

type outputWriter struct {
	runner *Runner
	id     string
}

func (w outputWriter) Write(b []byte) (int, error) { w.runner.output(w.id, b); return len(b), nil }

func (r *Runner) execute(ctx context.Context, s *runSession, req RunRequest, runtime RuntimeCandidate) {
	status, exit, message := "failed", -1, ""
	started := time.Now()
	defer func() {
		if ctx.Err() != nil {
			status = "stopped"
			message = "运行已停止"
		}
		s.cancel()
		r.mu.Lock()
		_ = windows.TerminateJobObject(s.job, 1)
		_ = windows.CloseHandle(s.job)
		r.active = nil
		r.mu.Unlock()
		if message == "" {
			message = fmt.Sprintf("退出码 %d · %.2f 秒", exit, time.Since(started).Seconds())
		}
		r.emit(RunEvent{ID: s.id, Kind: "exit", Phase: status, ExitCode: exit, Message: message})
		close(s.done)
	}()
	dir := filepath.Join(r.root, "workspaces", req.DraftID, req.ID)
	if err := os.MkdirAll(dir, 0700); err != nil {
		message = err.Error()
		return
	}
	source := filepath.Join(dir, "main"+extension(req.Language))
	if err := os.WriteFile(source, []byte(req.Code), 0600); err != nil {
		message = err.Error()
		return
	}
	program, args := runtime.Path, []string{}
	env := runtime.Env
	if env == nil {
		env = os.Environ()
	}
	env = append(append([]string{}, env...), "PYTHONIOENCODING=utf-8", "PYTHONUTF8=1")
	pathSet := false
	for i, value := range env {
		if key, old, ok := strings.Cut(value, "="); ok && strings.EqualFold(key, "PATH") {
			env[i] = "PATH=" + filepath.Dir(runtime.Path) + string(os.PathListSeparator) + old
			pathSet = true
			break
		}
	}
	if !pathSet {
		env = append(env, "PATH="+filepath.Dir(runtime.Path))
	}
	// MSVC 开发环境也可能携带旧 TEMP；编译和运行统一使用同级工作目录。
	filtered := env[:0]
	for _, value := range env {
		key, _, _ := strings.Cut(value, "=")
		switch strings.ToUpper(key) {
		case "TEMP", "TMP", "TMPDIR", "PYTHONDONTWRITEBYTECODE":
			continue
		}
		filtered = append(filtered, value)
	}
	env = append(filtered, "TEMP="+dir, "TMP="+dir, "TMPDIR="+dir, "PYTHONDONTWRITEBYTECODE=1")
	if runtime.Kind == "msvc" {
		env = append(env, "VSLANG=1033")
	}
	if req.Language != "python" {
		r.phase(s, "compiling")
		binary := filepath.Join(dir, "practice.exe")
		std := "c17"
		if req.Language == "cpp" {
			std = "c++17"
		}
		compileArgs := []string{"-std=" + std, "-Wall", "-Wextra", "-finput-charset=UTF-8", "-fexec-charset=UTF-8", filepath.Base(source), "-o", "practice.exe"}
		if runtime.Kind == "gcc" {
			// 部分 MinGW 工具仍按 ANSI 解析路径；相对文件名和固定中间产物避免传入中文绝对路径。
			compileArgs = append(compileArgs, "-save-temps=obj")
		}
		if runtime.Kind == "msvc" {
			compileArgs = []string{"/nologo", "/utf-8", "/W4", "/std:" + std, source, "/Fe:" + binary}
			if req.Language == "cpp" {
				compileArgs = append(compileArgs, "/EHsc")
			} else {
				compileArgs = append(compileArgs, "/TC")
			}
		}
		compileCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		cmd := hiddenCommand(compileCtx, runtime.Path, compileArgs...)
		cmd.Dir = dir
		cmd.Env = env
		cmd.Stdout = outputWriter{r, s.id}
		cmd.Stderr = outputWriter{r, s.id}
		if err := cmd.Start(); err != nil {
			cancel()
			message = "无法启动编译器：" + err.Error()
			return
		}
		if err := assignJob(s.job, cmd.Process.Pid); err != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			cancel()
			message = "无法管理编译进程：" + err.Error()
			return
		}
		err := cmd.Wait()
		timedOut := errors.Is(compileCtx.Err(), context.DeadlineExceeded)
		cancel()
		if err != nil {
			if cmd.ProcessState != nil {
				exit = cmd.ProcessState.ExitCode()
			}
			message = "编译失败，请检查上方输出"
			if timedOut {
				message = "编译超过 60 秒，已停止"
			}
			return
		}
		systemDirectory, err := windows.GetSystemDirectory()
		if err != nil {
			message = "定位系统命令处理器失败：" + err.Error()
			return
		}
		// 在新建的 ConPTY 内设置代码页；使用固定命令和文件名，不拼接草稿或用户路径。
		program = filepath.Join(systemDirectory, "cmd.exe")
		args = []string{"/d", "/s", "/c", "chcp 65001>nul && .\\practice.exe"}
	} else {
		args = []string{"-B", "-u", source}
	}
	if ctx.Err() != nil {
		return
	}
	terminal, err := pty.New()
	if err != nil {
		message = "创建交互终端失败：" + err.Error()
		return
	}
	r.mu.Lock()
	s.terminal = terminal
	r.mu.Unlock()
	cols, rows := req.Cols, req.Rows
	if cols < 20 {
		cols = 80
	}
	if rows < 3 {
		rows = 16
	}
	_ = terminal.Resize(cols, rows)
	cmd := terminal.Command(program, args...)
	cmd.Dir = dir
	cmd.Env = env
	if err = cmd.Start(); err != nil {
		_ = terminal.Close()
		message = "启动程序失败：" + err.Error()
		return
	}
	if err = assignJob(s.job, cmd.Process.Pid); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		_ = terminal.Close()
		message = "无法管理运行进程：" + err.Error()
		return
	}
	r.phase(s, "running")
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		buf := make([]byte, 16*1024)
		for {
			n, readErr := terminal.Read(buf)
			if n > 0 {
				r.output(s.id, buf[:n])
				if ctx.Err() == nil {
					time.Sleep(8 * time.Millisecond)
				}
			}
			if readErr != nil {
				if !errors.Is(readErr, io.EOF) && ctx.Err() == nil && !strings.Contains(strings.ToLower(readErr.Error()), "closed") {
					r.output(s.id, []byte("\r\n终端读取结束："+readErr.Error()+"\r\n"))
				}
				return
			}
		}
	}()
	err = cmd.Wait()
	r.mu.Lock()
	s.phase = "finishing"
	r.mu.Unlock()
	_ = windows.TerminateJobObject(s.job, 1)
	// Go PTY 的 Close 会立即关闭输出管道；先让 ConPTY 排空并由读取协程收到 EOF，避免丢掉末尾输出。
	con := terminal.(pty.ConPty)
	windows.ClosePseudoConsole(windows.Handle(con.Fd()))
	<-readDone
	if closeErr := errors.Join(con.InputPipe().Close(), con.OutputPipe().Close()); closeErr != nil && ctx.Err() == nil {
		message = "关闭终端失败：" + closeErr.Error()
	}
	if cmd.ProcessState != nil {
		exit = cmd.ProcessState.ExitCode()
	}
	status = "exited"
	if err != nil {
		status = "failed"
		message = "程序异常退出：" + err.Error()
	}
}

func (r *Runner) Write(id, text string) error {
	r.mu.Lock()
	if r.active == nil || r.active.id != id || r.active.terminal == nil || r.active.phase != "running" {
		r.mu.Unlock()
		return errors.New("当前没有等待输入的运行会话")
	}
	terminal := r.active.terminal
	r.mu.Unlock()
	_, err := terminal.Write([]byte(text))
	return err
}

func (r *Runner) Resize(id string, cols, rows int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.active == nil || r.active.id != id || r.active.terminal == nil || r.active.phase != "running" {
		return nil
	}
	if cols < 2 || rows < 2 || cols > 1000 || rows > 1000 {
		return errors.New("终端尺寸无效")
	}
	return r.active.terminal.Resize(cols, rows)
}

func (r *Runner) Stop(id string) error {
	r.mu.Lock()
	s := r.active
	if s == nil {
		r.mu.Unlock()
		return nil
	}
	if s.id != id {
		r.mu.Unlock()
		return errors.New("运行会话已经变化")
	}
	s.cancel()
	err := windows.TerminateJobObject(s.job, 1)
	r.mu.Unlock()
	if err != nil {
		return err
	}
	select {
	case <-s.done:
		return nil
	case <-time.After(5 * time.Second):
		return errors.New("停止进程超时，请重试")
	}
}

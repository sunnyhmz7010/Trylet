package trylet

import (
	"encoding/base64"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// 同一测试进程共用环境探测结果，各用例仍分别验证编译、交互和进程清理。
var installedRuntimeCandidates = sync.OnceValue(func() []RuntimeCandidate {
	return discoverRuntimes(Settings{})
})

func TestInstalledRuntimeMatrix(t *testing.T) {
	candidates := installedRuntimeCandidates()
	for _, c := range candidates {
		t.Logf("已验证候选 %s / %s: %s (%s)", c.Language, c.Kind, c.Path, c.Version)
	}
	for _, spec := range []struct{ language, kind, source, prompt, input, want string }{
		{"c", "gcc", "#include <stdio.h>\nint main(void){int n;printf(\"数字: \");scanf(\"%d\",&n);printf(\"中文结果=%d\\n\",n*2);return 0;}\n", "数字:", "21\r", "中文结果=42"},
		{"cpp", "gcc", "#include <iostream>\nint main(){int n;std::cout<<\"数字: \";std::cin>>n;std::cout<<\"中文结果=\"<<n*2<<std::endl;}\n", "数字:", "21\r", "中文结果=42"},
		{"python", "python", "n = int(input('数字: '))\nprint('中文结果=' + str(n * 2))\n", "数字:", "21\r", "中文结果=42"},
		{"c", "msvc", "#include <stdio.h>\nint main(void){int n;printf(\"数字: \");scanf(\"%d\",&n);printf(\"中文结果=%d\\n\",n*2);return 0;}\n", "数字:", "21\r", "中文结果=42"},
		{"cpp", "msvc", "#include <iostream>\nint main(){int n;std::cout<<\"数字: \";std::cin>>n;std::cout<<\"中文结果=\"<<n*2<<std::endl;}\n", "数字:", "21\r", "中文结果=42"},
		{"c", "clang", "#include <stdio.h>\nint main(void){puts(\"中文结果=42\");}\n", "", "", "中文结果=42"},
	} {
		t.Run(spec.language+"_"+spec.kind, func(t *testing.T) {
			t.Parallel()
			var candidate *RuntimeCandidate
			for _, c := range candidates {
				if c.Language == spec.language && c.Kind == spec.kind {
					value := c
					candidate = &value
					break
				}
			}
			if candidate == nil {
				t.Skip("本机未安装此环境")
			}
			runInteractiveFixture(t, *candidate, spec.source, spec.prompt, spec.input, spec.want)
		})
	}
}

func runInteractiveFixture(t *testing.T, candidate RuntimeCandidate, source, prompt, input, want string) string {
	t.Helper()
	events := make(chan RunEvent, 512)
	runner := NewRunner(filepath.Join(t.TempDir(), "中文 路径"), func(e RunEvent) { events <- e })
	id := newID()
	_, err := runner.Start(RunRequest{ID: id, DraftID: newID(), Language: candidate.Language, Code: source, Cols: 80, Rows: 15}, candidate)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = runner.Stop(id) })
	output := ""
	// ConPTY 可在文本中插入 CSI 模式切换；断言比较可见文字，失败日志仍保留原始输出。
	controlSequence := regexp.MustCompile("\x1b\\[[0-?]*[ -/]*[@-~]")
	sent := false
	timer := time.NewTimer(35 * time.Second)
	defer timer.Stop()
	for {
		select {
		case e := <-events:
			if e.Kind == "output" {
				b, _ := base64.StdEncoding.DecodeString(e.Data)
				output += string(b)
				if prompt != "" && strings.Contains(controlSequence.ReplaceAllString(output, ""), prompt) && !sent {
					if err = runner.Write(id, input); err != nil {
						t.Fatal(err)
					}
					sent = true
				}
			}
			if e.Kind == "exit" {
				if e.Phase != "exited" || e.ExitCode != 0 || !strings.Contains(controlSequence.ReplaceAllString(output, ""), want) {
					t.Fatalf("运行失败 %+v，output=%q", e, output)
				}
				return output
			}
		case <-timer.C:
			t.Fatalf("等待运行超时 output=%q", output)
		}
	}
}

func TestStopProcessTreeAndContinuousOutput(t *testing.T) {
	var python *RuntimeCandidate
	for _, c := range installedRuntimeCandidates() {
		if c.Language == "python" {
			value := c
			python = &value
			break
		}
	}
	if python == nil {
		t.Skip("本机没有 Python 3")
	}
	t.Run("输出缓存有界", func(t *testing.T) {
		events := make(chan RunEvent, 512)
		runner := NewRunner(t.TempDir(), func(e RunEvent) { events <- e })
		id := newID()
		_, err := runner.Start(RunRequest{ID: id, DraftID: newID(), Language: "python", Code: "i = 0\nwhile True:\n    print(str(i) + ' ' + chr(33 + i % 80) * 120, flush=True)\n    i += 1\n", Cols: 160, Rows: 80}, *python)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = runner.Stop(id) })
		count := 0
		timer := time.NewTimer(30 * time.Second)
		defer timer.Stop()
		for {
			select {
			case event := <-events:
				if event.Kind == "output" {
					data, _ := base64.StdEncoding.DecodeString(event.Data)
					count += len(data)
					if count > 1024*1024+2048 {
						t.Fatalf("输出超过上限：%d", count)
					}
					if strings.Contains(string(data), "后续输出已截断") {
						if err = runner.Stop(id); err != nil {
							t.Fatal(err)
						}
						return
					}
				}
				if event.Kind == "exit" {
					t.Fatalf("触发上限前退出，总输出=%d", count)
				}
			case <-timer.C:
				t.Fatalf("未触发输出上限，总输出=%d", count)
			}
		}
	})
	t.Run("子进程树", func(t *testing.T) {
		events := make(chan RunEvent, 512)
		runner := NewRunner(t.TempDir(), func(e RunEvent) { events <- e })
		id := newID()
		source := "import subprocess, sys, time\np = subprocess.Popen([sys.executable, '-c', 'import time; time.sleep(60)'], stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)\nprint('child:' + str(p.pid), flush=True)\ntime.sleep(60)\n"
		_, err := runner.Start(RunRequest{ID: id, DraftID: newID(), Code: source, Language: "python", Cols: 80, Rows: 15}, *python)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = runner.Stop(id) })
		output := ""
		child := 0
		timer := time.NewTimer(15 * time.Second)
		defer timer.Stop()
		re := regexp.MustCompile(`child:(\d+)`)
		for child == 0 {
			select {
			case e := <-events:
				if e.Kind == "output" {
					b, _ := base64.StdEncoding.DecodeString(e.Data)
					output += string(b)
					if m := re.FindStringSubmatch(output); len(m) > 0 {
						child, _ = strconv.Atoi(m[1])
					}
				}
			case <-timer.C:
				t.Fatalf("未收到子进程 PID：%q", output)
			}
		}
		handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(child))
		if err != nil {
			t.Fatal(err)
		}
		defer windows.CloseHandle(handle)
		started := time.Now()
		if err = runner.Stop(id); err != nil {
			t.Fatal(err)
		}
		if time.Since(started) > 5*time.Second {
			t.Fatal("停止响应超过 5 秒")
		}
		state, err := windows.WaitForSingleObject(handle, 3000)
		if err != nil || state != windows.WAIT_OBJECT_0 {
			t.Fatalf("子进程未退出，state=%d err=%v", state, err)
		}
	})
	t.Run("连续输出可停止", func(t *testing.T) {
		events := make(chan RunEvent, 512)
		runner := NewRunner(t.TempDir(), func(e RunEvent) { events <- e })
		id := newID()
		_, err := runner.Start(RunRequest{ID: id, DraftID: newID(), Language: "python", Code: "while True:\n    print('x' * 1000, flush=True)\n", Cols: 80, Rows: 15}, *python)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = runner.Stop(id) })
		timer := time.NewTimer(10 * time.Second)
		defer timer.Stop()
		bytes := 0
		for bytes < 30000 {
			select {
			case e := <-events:
				if e.Kind == "output" {
					b, _ := base64.StdEncoding.DecodeString(e.Data)
					bytes += len(b)
				}
				if e.Kind == "exit" {
					t.Fatal(fmt.Sprintf("程序提前退出：%+v", e))
				}
			case <-timer.C:
				t.Fatal("没有收到连续输出")
			}
		}
		if err = runner.Stop(id); err != nil {
			t.Fatal(err)
		}
	})
}

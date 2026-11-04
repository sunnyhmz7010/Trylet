package runner

import (
	"encoding/base64"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"trylet/internal/identity"
)

func TestRunnerInteractiveC(t *testing.T) {
	compiler, err := exec.LookPath("gcc.exe")
	if err != nil {
		t.Skip("本机没有 GCC")
	}
	events := make(chan RunEvent, 128)
	runner := NewRunner(t.TempDir(), func(e RunEvent) { events <- e })
	id := identity.New()
	code := "#include <stdio.h>\nint main(void){int n;printf(\"Number: \");scanf(\"%d\",&n);printf(\"answer=%d\\n\",n*2);return 0;}\n"
	_, err = runner.Start(RunRequest{ID: id, DraftID: identity.New(), Code: code, Language: "c", Cols: 80, Rows: 16}, RuntimeCandidate{Path: compiler, Kind: "gcc", Language: "c"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = runner.Stop(id) })
	output := ""
	inputSent := false
	timer := time.NewTimer(30 * time.Second)
	defer timer.Stop()
	for {
		select {
		case e := <-events:
			if e.Kind == "output" {
				b, _ := base64.StdEncoding.DecodeString(e.Data)
				output += string(b)
				if strings.Contains(output, "Number:") && !inputSent {
					if err = runner.Write(id, "21\r"); err != nil {
						t.Fatal(err)
					}
					inputSent = true
				}
			}
			if e.Kind == "exit" {
				if e.ExitCode != 0 || !strings.Contains(output, "answer=42") {
					t.Fatalf("%+v output=%q", e, output)
				}
				return
			}
		case <-timer.C:
			t.Fatalf("交互运行超时，output=%q", output)
		}
	}
}

func TestRunnerUsesPortableTempDirectory(t *testing.T) {
	var candidate RuntimeCandidate
	for _, found := range installedRuntimeCandidates() {
		if found.Language == "python" {
			candidate = found
			break
		}
	}
	if candidate.Path == "" {
		t.Skip("本机没有可用的 Python 3")
	}
	outside := filepath.Join(t.TempDir(), "outside-must-stay-absent")
	candidate.Env = append(os.Environ(), "TEMP="+outside, "TMP="+outside, "TMPDIR="+outside, "PYTHONDONTWRITEBYTECODE=0")
	source := "import os, sys, tempfile\nassert os.path.samefile(tempfile.gettempdir(), os.getcwd())\nassert os.environ['TMP'] == os.getcwd()\nassert sys.dont_write_bytecode\nwith open('portable_module.py', 'w', encoding='utf-8') as f: f.write('answer = 42\\n')\nimport portable_module\nassert not os.path.exists('__pycache__')\nwith tempfile.TemporaryFile() as f: f.write(b'local')\nprint('portable-temp-ok')\n"
	runInteractiveFixture(t, candidate, source, "", "", "portable-temp-ok")
	if _, err := os.Stat(outside); !os.IsNotExist(err) {
		t.Fatal("使用了候选环境携带的外部临时目录")
	}
}

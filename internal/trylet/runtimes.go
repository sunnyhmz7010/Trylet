package trylet

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

func hiddenCommand(ctx context.Context, name string, args ...string) *exec.Cmd {
	c := exec.CommandContext(ctx, name, args...)
	c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	return c
}

func probeRuntime(candidate RuntimeCandidate) (RuntimeCandidate, error) {
	if strings.Contains(strings.ToLower(candidate.Path), `\microsoft\windowsapps\`) {
		return candidate, errors.New("WindowsApps 占位入口")
	}
	if info, err := os.Stat(candidate.Path); err != nil || info.IsDir() {
		return candidate, errors.New("可执行文件不存在")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	args := []string{"--version"}
	if candidate.Kind == "msvc" {
		args = nil
	}
	if candidate.Language == "python" {
		args = []string{"-I", "-c", "import sys; print('TryletPython:' + sys.version.split()[0]); print(sys.executable)"}
	}
	cmd := hiddenCommand(ctx, candidate.Path, args...)
	if candidate.Env != nil {
		cmd.Env = candidate.Env
	}
	b, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(b))
	if ctx.Err() != nil {
		return candidate, ctx.Err()
	}
	if candidate.Language == "python" {
		if err != nil || !strings.Contains(text, "TryletPython:3.") {
			return candidate, errors.New("不是可用的 Python 3 解释器")
		}
		lines := strings.Split(text, "\n")
		for i, line := range lines {
			if strings.HasPrefix(line, "TryletPython:") {
				candidate.Version = strings.TrimPrefix(strings.TrimSpace(line), "TryletPython:")
				if i+1 < len(lines) {
					candidate.Path = strings.TrimSpace(lines[i+1])
				}
				break
			}
		}
	} else {
		if text == "" || (err != nil && candidate.Kind != "msvc") {
			return candidate, errors.New("编译器无法启动")
		}
		candidate.Version = strings.TrimSpace(strings.Split(text, "\n")[0])
	}
	candidate.ID = candidate.Language + ":" + strings.ToLower(candidate.Path)
	return candidate, nil
}

func discoverRuntimes(settings Settings) []RuntimeCandidate {
	var candidates []RuntimeCandidate
	add := func(lang, name, path, kind string) {
		if path != "" {
			candidates = append(candidates, RuntimeCandidate{Language: lang, Name: name, Path: path, Kind: kind})
		}
	}
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		for _, item := range []struct{ exe, lang, name, kind string }{{"python.exe", "python", "Python", "python"}, {"python3.exe", "python", "Python", "python"}, {"gcc.exe", "c", "GCC", "gcc"}, {"g++.exe", "cpp", "G++", "gcc"}, {"clang.exe", "c", "Clang", "clang"}, {"clang++.exe", "cpp", "Clang++", "clang"}} {
			p := filepath.Join(dir, item.exe)
			if _, err := os.Stat(p); err == nil {
				add(item.lang, item.name, p, item.kind)
			}
		}
	}
	for _, root := range []string{os.Getenv("VIRTUAL_ENV"), os.Getenv("CONDA_PREFIX"), filepath.Join(os.Getenv("USERPROFILE"), "miniconda3"), filepath.Join(os.Getenv("USERPROFILE"), "anaconda3"), filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "Python")} {
		if root == "" {
			continue
		}
		add("python", "Python", filepath.Join(root, "python.exe"), "python")
		add("python", "Python venv", filepath.Join(root, "Scripts", "python.exe"), "python")
		matches, _ := filepath.Glob(filepath.Join(root, "Python*", "python.exe"))
		for _, match := range matches {
			add("python", "Python", match, "python")
		}
	}
	for _, hive := range []registry.Key{registry.CURRENT_USER, registry.LOCAL_MACHINE} {
		for _, view := range []uint32{registry.WOW64_64KEY, registry.WOW64_32KEY} {
			k, err := registry.OpenKey(hive, `Software\Python\PythonCore`, registry.READ|view)
			if err != nil {
				continue
			}
			versions, _ := k.ReadSubKeyNames(-1)
			_ = k.Close()
			for _, version := range versions {
				install, err := registry.OpenKey(hive, `Software\Python\PythonCore\`+version+`\InstallPath`, registry.READ|view)
				if err != nil {
					continue
				}
				p, _, _ := install.GetStringValue("ExecutablePath")
				if p == "" {
					base, _, _ := install.GetStringValue("")
					p = filepath.Join(base, "python.exe")
				}
				_ = install.Close()
				add("python", "Python "+version, p, "python")
			}
		}
	}
	if launcher, err := exec.LookPath("py.exe"); err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		b, _ := hiddenCommand(ctx, launcher, "--list-paths").CombinedOutput()
		cancel()
		re := regexp.MustCompile(`(?im)([a-z]:\\[^\r\n]*python(?:[0-9.]*)?\.exe)\s*$`)
		for _, match := range re.FindAllStringSubmatch(string(b), -1) {
			add("python", "Python Launcher", strings.TrimSpace(match[1]), "python")
		}
	}
	for language, p := range settings.CustomPaths {
		kind := "gcc"
		name := "自定义编译器"
		switch {
		case language == "python":
			kind = "python"
			name = "自定义 Python"
		case strings.EqualFold(filepath.Base(p), "cl.exe"):
			kind = "msvc"
		case strings.Contains(strings.ToLower(filepath.Base(p)), "clang"):
			kind = "clang"
		}
		add(language, name, p, kind)
	}
	candidates = append(candidates, discoverMSVC()...)
	seen := map[string]int{}
	unique := make([]RuntimeCandidate, 0, len(candidates))
	for _, c := range candidates {
		key := c.Language + strings.ToLower(c.Path)
		if index, exists := seen[key]; exists {
			if len(unique[index].Env) == 0 && len(c.Env) > 0 {
				unique[index].Env = c.Env
			}
		} else {
			seen[key] = len(unique)
			unique = append(unique, c)
		}
	}
	var mu sync.Mutex
	var wg sync.WaitGroup
	gate := make(chan struct{}, 6)
	result := []RuntimeCandidate{}
	verifiedIDs := map[string]bool{}
	for _, c := range unique {
		wg.Add(1)
		go func(c RuntimeCandidate) {
			defer wg.Done()
			gate <- struct{}{}
			defer func() { <-gate }()
			verified, err := probeRuntime(c)
			if err == nil {
				mu.Lock()
				if !verifiedIDs[verified.ID] {
					result = append(result, verified)
					verifiedIDs[verified.ID] = true
				}
				mu.Unlock()
			}
		}(c)
	}
	wg.Wait()
	sort.Slice(result, func(i, j int) bool {
		if result[i].Language != result[j].Language {
			return result[i].Language < result[j].Language
		}
		return result[i].Path < result[j].Path
	})
	return result
}

func discoverMSVC() []RuntimeCandidate {
	vswhere := filepath.Join(os.Getenv("ProgramFiles(x86)"), "Microsoft Visual Studio", "Installer", "vswhere.exe")
	if _, err := os.Stat(vswhere); err != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	b, err := hiddenCommand(ctx, vswhere, "-products", "*", "-requires", "Microsoft.VisualStudio.Component.VC.Tools.x86.x64", "-format", "json", "-utf8").Output()
	if err != nil {
		return nil
	}
	var installs []struct {
		Path string `json:"installationPath"`
	}
	if json.Unmarshal(b, &installs) != nil {
		return nil
	}
	var out []RuntimeCandidate
	for _, install := range installs {
		matches, _ := filepath.Glob(filepath.Join(install.Path, "VC", "Tools", "MSVC", "*", "bin", "Hostx64", "x64", "cl.exe"))
		if len(matches) == 0 {
			continue
		}
		sort.Strings(matches)
		compiler := matches[len(matches)-1]
		script := filepath.Join(install.Path, "Common7", "Tools", "VsDevCmd.bat")
		cmdline := `""` + script + `" -no_logo -arch=x64 -host_arch=x64 >nul && set"`
		cmd := hiddenCommand(ctx, os.Getenv("COMSPEC"), "/d", "/s", "/c", cmdline)
		// cmd.exe 的 /c 参数使用自己的引号规则，不能经过通用的 CommandLineToArgvW 转义。
		cmd.SysProcAttr.CmdLine = windows.ComposeCommandLine([]string{os.Getenv("COMSPEC"), "/d", "/s", "/c"}) + " " + cmdline
		envBytes, err := cmd.Output()
		if err != nil {
			continue
		}
		env := []string{}
		for _, line := range strings.Split(strings.ReplaceAll(string(envBytes), "\r\n", "\n"), "\n") {
			if strings.Contains(line, "=") {
				env = append(env, line)
			}
		}
		for _, lang := range []string{"c", "cpp"} {
			out = append(out, RuntimeCandidate{Language: lang, Name: "MSVC", Path: compiler, Kind: "msvc", Env: env})
		}
	}
	return out
}

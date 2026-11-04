package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"

	"trylet/internal/language"
	winprocess "trylet/internal/process"
	storage "trylet/internal/store"
)

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
	cmd := winprocess.Command(ctx, candidate.Path, args...)
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
		if err := probeCompiler(candidate); err != nil {
			return candidate, err
		}
	}
	candidate.ID = candidate.Language + ":" + strings.ToLower(candidate.Path)
	return candidate, nil
}

func probeCompiler(candidate RuntimeCandidate) error {
	dir, err := os.MkdirTemp("", "trylet-probe-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	source := "main" + language.Extension(candidate.Language)
	code := "#include <stdio.h>\nint main(void) { puts(\"TryletRuntime\"); return 0; }\n"
	if candidate.Language == "cpp" {
		code = "#include <iostream>\nint main() { std::cout << \"TryletRuntime\" << std::endl; }\n"
	}
	if err := os.WriteFile(filepath.Join(dir, source), []byte(code), 0600); err != nil {
		return err
	}
	job, err := winprocess.CreateJob()
	if err != nil {
		return err
	}
	defer windows.CloseHandle(job)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	args := compileArguments(candidate.Language, candidate.Kind, source, "probe.exe")
	cmd := winprocess.Command(ctx, candidate.Path, args...)
	cmd.Dir, cmd.Env = dir, runtimeEnvironment(candidate, dir)
	// 编译器会派生链接器；超时和退出时一起回收，避免探测残留后台进程。
	cmd.Cancel = func() error { return windows.TerminateJobObject(job, 1) }
	cmd.WaitDelay = time.Second
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("试编译无法启动：%w", err)
	}
	if err := winprocess.AssignJob(job, cmd.Process.Pid); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return err
	}
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("试编译失败：%w：%s", err, strings.TrimSpace(output.String()))
	}
	cmd = winprocess.Command(ctx, filepath.Join(dir, "probe.exe"))
	cmd.Dir, cmd.Env = dir, runtimeEnvironment(candidate, dir)
	b, err := cmd.CombinedOutput()
	if err != nil || strings.TrimSpace(string(b)) != "TryletRuntime" {
		return fmt.Errorf("试运行未通过：%v：%s", err, strings.TrimSpace(string(b)))
	}
	return nil
}

func collectRuntimeCandidates(settings storage.Settings) []RuntimeCandidate {
	var candidates []RuntimeCandidate
	join := func(root string, parts ...string) string {
		root = strings.Trim(strings.TrimSpace(root), `"`)
		if !filepath.IsAbs(root) {
			return ""
		}
		return filepath.Join(append([]string{root}, parts...)...)
	}
	add := func(lang, name, path, kind string) {
		path = strings.Trim(strings.TrimSpace(path), `"`)
		if !filepath.IsAbs(path) {
			return
		}
		path = filepath.Clean(path)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			candidates = append(candidates, RuntimeCandidate{Language: lang, Name: name, Path: path, Kind: kind})
		}
	}
	addPython := func(root string) {
		add("python", "Python", join(root, "python.exe"), "python")
		add("python", "Python venv", join(root, "Scripts", "python.exe"), "python")
	}
	var condaRoots []string
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		for _, item := range []struct{ exe, lang, name, kind string }{{"python.exe", "python", "Python", "python"}, {"python3.exe", "python", "Python", "python"}, {"gcc.exe", "c", "GCC", "gcc"}, {"g++.exe", "cpp", "G++", "gcc"}, {"clang.exe", "c", "Clang", "clang"}, {"clang++.exe", "cpp", "Clang++", "clang"}} {
			add(item.lang, item.name, join(dir, item.exe), item.kind)
		}
		for _, exe := range []string{"conda.exe", "conda.bat"} {
			if p := join(dir, exe); p != "" {
				if _, err := os.Stat(p); err == nil {
					condaRoots = append(condaRoots, filepath.Dir(filepath.Dir(p)))
				}
			}
		}
	}
	home, local := os.Getenv("USERPROFILE"), os.Getenv("LOCALAPPDATA")
	systemDrive := os.Getenv("SystemDrive")
	if systemDrive != "" {
		systemDrive += string(os.PathSeparator)
	}
	scoopRoots := []string{os.Getenv("SCOOP"), join(home, "scoop"), os.Getenv("SCOOP_GLOBAL"), join(os.Getenv("ProgramData"), "scoop")}
	pythonRoots := []string{os.Getenv("VIRTUAL_ENV"), os.Getenv("CONDA_PREFIX"), join(local, "Programs", "Python")}
	pythonParents := []string{os.Getenv("WORKON_HOME"), join(home, ".virtualenvs"), join(home, ".conda", "envs"), join(os.Getenv("PYENV_ROOT"), "versions"), join(home, ".pyenv", "pyenv-win", "versions"), os.Getenv("UV_PYTHON_INSTALL_DIR"), join(os.Getenv("APPDATA"), "uv", "data", "python"), join(os.Getenv("APPDATA"), "uv", "python"), join(local, "uv", "python")}
	pythonParents = append(pythonParents, filepath.SplitList(os.Getenv("CONDA_ENVS_PATH"))...)
	for _, base := range []string{home, local, join(local, "Programs"), os.Getenv("ProgramData"), os.Getenv("ProgramFiles"), systemDrive} {
		for _, name := range []string{"miniconda3", "anaconda3", "miniforge3", "mambaforge"} {
			condaRoots = append(condaRoots, join(base, name))
		}
	}
	if exe := os.Getenv("CONDA_EXE"); filepath.IsAbs(exe) {
		condaRoots = append(condaRoots, filepath.Dir(filepath.Dir(exe)))
	}
	for _, root := range scoopRoots {
		for _, name := range []string{"miniconda3", "anaconda3", "miniforge3", "mambaforge"} {
			condaRoots = append(condaRoots, join(root, "apps", name, "current"))
		}
		if pattern := join(root, "apps", "python*", "current", "python.exe"); pattern != "" {
			matches, _ := filepath.Glob(pattern)
			for _, match := range matches {
				addPython(filepath.Dir(match))
			}
		}
	}
	for _, root := range condaRoots {
		addPython(root)
		pythonParents = append(pythonParents, join(root, "envs"))
	}
	// Conda 的登记清单可以包含其他盘符的未激活环境，无需启动 Conda 或扫描磁盘。
	if path := join(home, ".conda", "environments.txt"); path != "" {
		if b, err := os.ReadFile(path); err == nil {
			for _, root := range strings.Split(strings.TrimPrefix(string(b), "\ufeff"), "\n") {
				addPython(strings.TrimSpace(root))
			}
		}
	}
	for _, base := range []string{join(local, "Programs", "Python"), os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)"), systemDrive} {
		if pattern := join(base, "Python*", "python.exe"); pattern != "" {
			matches, _ := filepath.Glob(pattern)
			for _, match := range matches {
				addPython(filepath.Dir(match))
			}
		}
	}
	workingDir, _ := os.Getwd()
	executable, _ := os.Executable()
	for _, base := range []string{workingDir, filepath.Dir(executable)} {
		for _, name := range []string{".venv", "venv", "env"} {
			pythonRoots = append(pythonRoots, join(base, name))
		}
	}
	for _, root := range pythonRoots {
		addPython(root)
	}
	for _, parent := range pythonParents {
		for _, suffix := range [][]string{{"*", "python.exe"}, {"*", "Scripts", "python.exe"}, {"*", "install", "python.exe"}} {
			if pattern := join(parent, suffix...); pattern != "" {
				matches, _ := filepath.Glob(pattern)
				for _, match := range matches {
					add("python", "Python", match, "python")
				}
			}
		}
	}
	for _, hive := range []registry.Key{registry.CURRENT_USER, registry.LOCAL_MACHINE} {
		for _, view := range []uint32{registry.WOW64_64KEY, registry.WOW64_32KEY} {
			k, err := registry.OpenKey(hive, `Software\Python`, registry.READ|view)
			if err != nil {
				continue
			}
			companies, _ := k.ReadSubKeyNames(-1)
			_ = k.Close()
			for _, company := range companies {
				key, err := registry.OpenKey(hive, `Software\Python\`+company, registry.READ|view)
				if err != nil {
					continue
				}
				versions, _ := key.ReadSubKeyNames(-1)
				_ = key.Close()
				for _, version := range versions {
					install, err := registry.OpenKey(hive, `Software\Python\`+company+`\`+version+`\InstallPath`, registry.READ|view)
					if err != nil {
						continue
					}
					p, _, _ := install.GetStringValue("ExecutablePath")
					args, _, _ := install.GetStringValue("ExecutableArguments")
					if p == "" && company == "PythonCore" {
						base, _, _ := install.GetStringValue("")
						p = join(base, "python.exe")
					}
					_ = install.Close()
					if strings.TrimSpace(args) == "" {
						add("python", company+" "+version, p, "python")
					}
				}
			}
		}
	}
	if launcher, err := exec.LookPath("py.exe"); err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		b, _ := winprocess.Command(ctx, launcher, "--list-paths").CombinedOutput()
		cancel()
		re := regexp.MustCompile(`(?im)((?:[a-z]:\\|\\\\)[^\r\n]*python[0-9.]*t?\.exe)\s*$`)
		for _, match := range re.FindAllStringSubmatch(string(b), -1) {
			add("python", "Python Launcher", strings.TrimSpace(match[1]), "python")
		}
	}
	compilerRoots := []string{join(systemDrive, "mingw64"), join(systemDrive, "mingw32"), join(systemDrive, "MinGW"), join(systemDrive, "llvm-mingw")}
	for _, base := range []string{systemDrive, home, join(local, "Programs"), os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)")} {
		for _, name := range []string{"msys64", "msys32", "LLVM", filepath.Join("CodeBlocks", "MinGW")} {
			compilerRoots = append(compilerRoots, join(base, name))
		}
	}
	for _, root := range scoopRoots {
		for _, name := range []string{"gcc", "mingw", "llvm", "llvm-mingw", "msys2"} {
			compilerRoots = append(compilerRoots, join(root, "apps", name, "current"))
		}
	}
	chocolatey := os.Getenv("ChocolateyInstall")
	if chocolatey == "" {
		chocolatey = join(os.Getenv("ProgramData"), "chocolatey")
	}
	for _, root := range []string{join(chocolatey, "lib", "mingw", "tools", "install", "mingw64"), join(chocolatey, "lib", "mingw", "tools", "install", "mingw32")} {
		compilerRoots = append(compilerRoots, root)
	}
	for _, root := range compilerRoots {
		for _, subdir := range []string{"bin", "ucrt64/bin", "mingw64/bin", "mingw32/bin", "clang64/bin", "clangarm64/bin"} {
			for _, item := range []struct{ exe, lang, name, kind string }{{"gcc.exe", "c", "GCC", "gcc"}, {"g++.exe", "cpp", "G++", "gcc"}, {"clang.exe", "c", "Clang", "clang"}, {"clang++.exe", "cpp", "Clang++", "clang"}} {
				add(item.lang, item.name, join(root, subdir, item.exe), item.kind)
			}
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
	candidates = append(candidates, discoverMSVC(settings.CustomPaths)...)
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
	return unique
}

func DiscoverRuntimes(settings storage.Settings) []RuntimeCandidate {
	unique := collectRuntimeCandidates(settings)
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

func discoverMSVC(customPaths map[string]string) []RuntimeCandidate {
	var roots []string
	vswhere := filepath.Join(os.Getenv("ProgramFiles(x86)"), "Microsoft Visual Studio", "Installer", "vswhere.exe")
	if binary, err := exec.LookPath("vswhere.exe"); err == nil {
		vswhere = binary
	}
	if _, err := os.Stat(vswhere); err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		b, err := winprocess.Command(ctx, vswhere, "-products", "*", "-prerelease", "-format", "json", "-utf8").Output()
		cancel()
		var installs []struct {
			Path string `json:"installationPath"`
		}
		if err == nil && json.Unmarshal(b, &installs) == nil {
			for _, install := range installs {
				roots = append(roots, install.Path)
			}
		}
	}
	for _, base := range []string{os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)")} {
		if !filepath.IsAbs(base) {
			continue
		}
		matches, _ := filepath.Glob(filepath.Join(base, "Microsoft Visual Studio", "*", "*", "Common7", "Tools", "VsDevCmd.bat"))
		for _, match := range matches {
			roots = append(roots, filepath.Dir(filepath.Dir(filepath.Dir(match))))
		}
	}
	for _, path := range customPaths {
		if !filepath.IsAbs(path) || !strings.EqualFold(filepath.Base(path), "cl.exe") {
			continue
		}
		for dir, depth := filepath.Dir(path), 0; depth < 8; dir, depth = filepath.Dir(dir), depth+1 {
			if _, err := os.Stat(filepath.Join(dir, "Common7", "Tools", "VsDevCmd.bat")); err == nil {
				roots = append(roots, dir)
				break
			}
		}
	}
	architectures := [][2]string{{"x64", "x64"}, {"x86", "x64"}, {"x86", "x86"}}
	if runtime.GOARCH == "arm64" {
		architectures = append([][2]string{{"arm64", "arm64"}, {"x64", "arm64"}, {"x86", "arm64"}}, architectures...)
	}
	seen := map[string]bool{}
	var out []RuntimeCandidate
	for _, root := range roots {
		key := strings.ToLower(filepath.Clean(root))
		if !filepath.IsAbs(root) || seen[key] {
			continue
		}
		seen[key] = true
		compiler, host, target := "", "", ""
		for _, arch := range architectures {
			matches, _ := filepath.Glob(filepath.Join(root, "VC", "Tools", "MSVC", "*", "bin", "Host"+arch[0], arch[1], "cl.exe"))
			if len(matches) > 0 {
				sort.Strings(matches)
				compiler, host, target = matches[len(matches)-1], arch[0], arch[1]
				break
			}
		}
		if compiler == "" {
			continue
		}
		script := filepath.Join(root, "Common7", "Tools", "VsDevCmd.bat")
		cmdline := `""` + script + `" -no_logo -arch=` + target + ` -host_arch=` + host + ` >nul && set"`
		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
		cmd := winprocess.Command(ctx, os.Getenv("COMSPEC"), "/d", "/s", "/c", cmdline)
		// cmd.exe 的 /c 参数使用自己的引号规则，不能经过通用的 CommandLineToArgvW 转义。
		cmd.SysProcAttr.CmdLine = windows.ComposeCommandLine([]string{os.Getenv("COMSPEC"), "/d", "/s", "/c"}) + " " + cmdline
		envBytes, err := cmd.Output()
		cancel()
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
		llvmDirs := []string{filepath.Join(root, "VC", "Tools", "Llvm", "bin"), filepath.Join(root, "VC", "Tools", "Llvm", host, "bin")}
		for _, dir := range llvmDirs {
			for _, spec := range []struct{ exe, lang string }{{"clang.exe", "c"}, {"clang++.exe", "cpp"}} {
				path := filepath.Join(dir, spec.exe)
				if info, err := os.Stat(path); err == nil && !info.IsDir() {
					out = append(out, RuntimeCandidate{Language: spec.lang, Name: "Clang", Path: path, Kind: "clang", Env: env})
				}
			}
		}
	}
	return out
}

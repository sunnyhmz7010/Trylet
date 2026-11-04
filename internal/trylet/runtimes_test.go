package trylet

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func isolateRuntimePaths(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, key := range []string{"PATH", "VIRTUAL_ENV", "CONDA_PREFIX", "CONDA_EXE", "CONDA_ENVS_PATH", "WORKON_HOME", "PYENV_ROOT", "UV_PYTHON_INSTALL_DIR", "SCOOP_GLOBAL", "ChocolateyInstall"} {
		t.Setenv(key, "")
	}
	for key, dir := range map[string]string{"USERPROFILE": "home", "LOCALAPPDATA": "local", "APPDATA": "roaming", "ProgramData": "data", "ProgramFiles": "programs", "ProgramFiles(x86)": "programs-x86", "SCOOP": "scoop"} {
		t.Setenv(key, filepath.Join(root, dir))
	}
	t.Setenv("SystemDrive", root)
	t.Chdir(root)
	return root
}

func writeRuntimeFixture(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestCollectRuntimeCandidatesFindsUnactivatedInstallations(t *testing.T) {
	root := isolateRuntimePaths(t)
	conda := filepath.Join(root, "other drive", "research", "python.exe")
	fixtures := []struct{ language, path string }{
		{"python", filepath.Join(root, "home", "miniconda3", "envs", "inactive", "python.exe")},
		{"python", filepath.Join(root, "home", "miniforge3", "python.exe")},
		{"python", conda},
		{"python", filepath.Join(root, "conda envs", "analysis", "python.exe")},
		{"python", filepath.Join(root, "home", ".virtualenvs", "lesson", "Scripts", "python.exe")},
		{"python", filepath.Join(root, ".venv", "Scripts", "python.exe")},
		{"python", filepath.Join(root, "local", "Programs", "Python", "Python313", "python.exe")},
		{"python", filepath.Join(root, "home", ".pyenv", "pyenv-win", "versions", "3.13", "python.exe")},
		{"python", filepath.Join(root, "local", "uv", "python", "cpython-3.13", "install", "python.exe")},
		{"python", filepath.Join(root, "roaming", "uv", "data", "python", "cpython-3.14", "python.exe")},
		{"python", filepath.Join(root, "scoop", "apps", "python313", "current", "python.exe")},
		{"c", filepath.Join(root, "msys64", "ucrt64", "bin", "gcc.exe")},
		{"cpp", filepath.Join(root, "msys64", "clangarm64", "bin", "clang++.exe")},
		{"c", filepath.Join(root, "programs", "LLVM", "bin", "clang.exe")},
		{"cpp", filepath.Join(root, "scoop", "apps", "gcc", "current", "bin", "g++.exe")},
		{"c", filepath.Join(root, "data", "chocolatey", "lib", "mingw", "tools", "install", "mingw64", "bin", "gcc.exe")},
		{"c", filepath.Join(root, "quoted path", "gcc.exe")},
	}
	for _, fixture := range fixtures {
		writeRuntimeFixture(t, fixture.path, "")
	}
	writeRuntimeFixture(t, filepath.Join(root, "home", ".conda", "environments.txt"), "\ufeff"+filepath.Dir(conda)+"\r\n\r\n"+filepath.Join(root, "removed-env")+"\r\n")
	t.Setenv("CONDA_ENVS_PATH", filepath.Join(root, "conda envs"))
	quoted := filepath.Join(root, "quoted path")
	t.Setenv("PATH", `"`+quoted+`";`+quoted+";;")
	writeRuntimeFixture(t, filepath.Join(root, "python.exe"), "")
	writeRuntimeFixture(t, filepath.Join(root, "projects", "nested", ".venv", "Scripts", "python.exe"), "")
	found := map[string]int{}
	for _, candidate := range collectRuntimeCandidates(Settings{CustomPaths: map[string]string{"c": filepath.Join(quoted, "gcc.exe")}}) {
		key := candidate.Language + ":" + strings.ToLower(candidate.Path)
		found[key]++
		if found[key] != 1 {
			t.Errorf("同一候选被重复收集：%s", candidate.Path)
		}
	}
	for _, fixture := range fixtures {
		if found[fixture.language+":"+strings.ToLower(fixture.path)] != 1 {
			t.Errorf("未找到未激活或未加入 PATH 的环境：%s", fixture.path)
		}
	}
	for _, path := range []string{filepath.Join(root, "python.exe"), filepath.Join(root, "projects", "nested", ".venv", "Scripts", "python.exe")} {
		if found["python:"+strings.ToLower(path)] != 0 {
			t.Errorf("扫描超出了已知目录，或把空 PATH 当作当前目录：%s", path)
		}
	}
}

func TestDiscoverMSVCWithoutInstaller(t *testing.T) {
	for _, custom := range []bool{false, true} {
		name := "标准安装目录"
		if custom {
			name = "自定义安装目录"
		}
		t.Run(name, func(t *testing.T) {
			root := isolateRuntimePaths(t)
			install := filepath.Join(root, "programs", "Microsoft Visual Studio", "2026", "BuildTools")
			if custom {
				install = filepath.Join(root, "custom tools", "VS")
			}
			host := "x64"
			if runtime.GOARCH == "arm64" {
				host = "arm64"
			}
			compiler := filepath.Join(install, "VC", "Tools", "MSVC", "14.50.12345", "bin", "Host"+host, host, "cl.exe")
			writeRuntimeFixture(t, compiler, "")
			writeRuntimeFixture(t, filepath.Join(install, "Common7", "Tools", "VsDevCmd.bat"), "@echo off\r\nset TRYLET_TEST_DEV_ENV=loaded\r\n")
			clang := filepath.Join(install, "VC", "Tools", "Llvm", host, "bin", "clang.exe")
			writeRuntimeFixture(t, clang, "")
			paths := map[string]string{}
			if custom {
				paths["cpp"] = compiler
			}
			candidates := discoverMSVC(paths)
			if len(candidates) != 3 {
				t.Fatalf("缺少 MSVC C/C++ 或附带的 Clang：%+v", candidates)
			}
			for _, candidate := range candidates {
				if !slices.Contains(candidate.Env, "TRYLET_TEST_DEV_ENV=loaded") {
					t.Fatalf("未加载对应的开发环境：%+v", candidate)
				}
			}
		})
	}
}

func TestProbeRuntimeRejectsVersionOnlyExecutable(t *testing.T) {
	root := t.TempDir()
	for _, key := range []string{"TEMP", "TMP", "TMPDIR"} {
		t.Setenv(key, root)
	}
	// cmd.exe 无参数能输出版本横幅，但不具备编译能力。
	_, err := probeRuntime(RuntimeCandidate{Language: "c", Kind: "msvc", Path: os.Getenv("COMSPEC")})
	if err == nil || (!strings.Contains(err.Error(), "试编译") && !strings.Contains(err.Error(), "试运行")) {
		t.Fatalf("仅输出版本的程序未被编译和运行验证拒绝：%v", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("检测失败后留下了临时产物：%v %v", entries, err)
	}
}

func TestProbeRuntimeRejectsMissingMSVCHeaders(t *testing.T) {
	for _, candidate := range installedRuntimeCandidates() {
		if candidate.Language != "c" || candidate.Kind != "msvc" {
			continue
		}
		candidate.Env = append(append([]string{}, candidate.Env...), "INCLUDE="+filepath.Join(t.TempDir(), "missing-headers"))
		if _, err := probeRuntime(candidate); err == nil || !strings.Contains(err.Error(), "试编译") {
			t.Fatalf("缺少标准头文件的编译器仍被判为可用：%v", err)
		}
		return
	}
	t.Skip("本机没有 MSVC")
}

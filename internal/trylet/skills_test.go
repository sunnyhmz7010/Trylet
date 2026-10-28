package trylet

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"golang.org/x/sys/windows"
)

type skillTestRemote struct {
	mu       sync.Mutex
	files    map[string]string
	revision string
	fail     bool
	private  bool
}

func skillTestArchive(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	z := zip.NewWriter(&buffer)
	for name, content := range files {
		f, err := z.Create("catalog-version/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = f.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func skillTestService(t *testing.T) (*Service, *skillTestRemote, SkillLocation) {
	t.Helper()
	dir := t.TempDir()
	store, err := NewStore(dir, dir)
	if err != nil {
		t.Fatal(err)
	}
	s := NewService(store, nil, nil)
	s.skillHome = filepath.Join(dir, "user")
	remote := &skillTestRemote{revision: strings.Repeat("a", 40), files: map[string]string{"skills/alpha/SKILL.md": "alpha v1", "skills/alpha/scripts/run.py": "print(1)", "skills/beta/SKILL.md": "beta v1", "shared.txt": "shared"}}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		remote.mu.Lock()
		defer remote.mu.Unlock()
		if remote.fail {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		if remote.private && r.Header.Get("Authorization") != "Bearer local-test-token" {
			http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
			return
		}
		switch r.URL.Path {
		case "/repos/acme/catalog":
			fmt.Fprint(w, `{"default_branch":"main"}`)
		case "/repos/acme/catalog/commits/main":
			fmt.Fprintf(w, `{"sha":%q}`, remote.revision)
		case "/repos/acme/catalog/zipball/" + remote.revision:
			w.Header().Set("Content-Type", "application/zip")
			_, _ = w.Write(skillTestArchive(t, remote.files))
		default:
			http.NotFound(w, r)
		}
	}))
	server.Listener, err = net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server.Start()
	t.Cleanup(server.Close)
	s.publisher.base = server.URL
	t.Cleanup(func() {
		for id := range s.skillDownloads {
			s.removeSkillDownload(id)
		}
	})
	workspace, err := s.GetSkillWorkspace()
	if err != nil {
		t.Fatal(err)
	}
	return s, remote, workspace.Locations[0]
}

func skillTestInstall(t *testing.T, s *Service, location SkillLocation, extract bool, paths ...string) []SkillInstallResult {
	t.Helper()
	preview, err := s.PrepareSkillInstall("https://github.com/acme/catalog.git", extract)
	if err != nil {
		t.Fatal(err)
	}
	results, err := s.InstallSkills(preview.ID, location.ID, paths)
	if err != nil {
		t.Fatal(err)
	}
	for _, result := range results {
		if result.Error != "" || result.Warning != "" {
			t.Fatalf("安装失败：%+v", result)
		}
	}
	s.DiscardSkillPreview(preview.ID)
	return results
}

func skillTestRead(t *testing.T, path, expected string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil || string(data) != expected {
		t.Fatalf("文件 %s：got %q, err %v, want %q", path, data, err, expected)
	}
}

func TestSkillsLocationsScanAndSelectedInstall(t *testing.T) {
	s, _, location := skillTestService(t)
	items, err := s.ScanSkills(location.ID)
	if err != nil || len(items) != 0 {
		t.Fatalf("空位置扫描：%+v %v", items, err)
	}
	if _, err := os.Stat(location.Path); !os.IsNotExist(err) {
		t.Fatal("浏览不应创建目录")
	}
	preview, err := s.PrepareSkillInstall("acme/catalog", true)
	if err != nil || len(preview.Candidates) != 2 {
		t.Fatalf("识别嵌套技能：%+v %v", preview, err)
	}
	results, err := s.InstallSkills(preview.ID, location.ID, []string{"skills/alpha"})
	if err != nil || len(results) != 1 || results[0].Error != "" {
		t.Fatalf("勾选安装：%+v %v", results, err)
	}
	skillTestRead(t, filepath.Join(location.Path, "alpha", "scripts", "run.py"), "print(1)")
	if _, err := os.Stat(filepath.Join(location.Path, "beta")); !os.IsNotExist(err) {
		t.Fatal("未勾选的技能不应安装")
	}
	results, err = s.InstallSkills(preview.ID, location.ID, []string{"skills/alpha", "skills/beta"})
	if err != nil || len(results) != 2 || results[0].Error == "" || results[1].Error != "" {
		t.Fatalf("同名保护和逐项结果：%+v %v", results, err)
	}
	skillTestRead(t, filepath.Join(location.Path, "alpha", "SKILL.md"), "alpha v1")
	reopened, err := NewStore(s.store.root, s.store.tempDir)
	if err != nil {
		t.Fatal(err)
	}
	if got := reopened.skillsState().Bindings[skillPathID(filepath.Join(location.Path, "alpha"))]; got.Repository != "acme/catalog" || got.SourcePath != "skills/alpha" || got.Files["SKILL.md"] == "" {
		t.Fatalf("仓库绑定未持久化：%+v", got)
	}
	project := filepath.Join(s.store.root, "project")
	custom := filepath.Join(project, "other-skills")
	workspace, err := s.SaveSkillPreferences(SkillPreferences{Scope: "project", Project: project, Location: custom, CustomLocations: []SkillCustomLocation{{Path: custom, Project: project}}})
	if err != nil || len(workspace.Locations) != 7 || workspace.Preferences.Location != custom {
		t.Fatalf("项目级与自定义目录：%+v %v", workspace, err)
	}
	if _, err := os.Stat(project); !os.IsNotExist(err) {
		t.Fatal("选择位置不应创建项目目录")
	}
	projectLocation := workspace.Locations[3]
	skillTestInstall(t, s, projectLocation, true, "skills/alpha")
	projectItems, err := s.ScanSkills(projectLocation.ID)
	if err != nil || len(projectItems) != 1 || projectItems[0].ID == skillPathID(filepath.Join(location.Path, "alpha")) {
		t.Fatalf("项目安装应与用户安装隔离：%+v %v", projectItems, err)
	}
}

func TestSkillsWholeRepositoryExistingAndSystemProtection(t *testing.T) {
	s, _, location := skillTestService(t)
	skillTestInstall(t, s, location, false, ".")
	skillTestRead(t, filepath.Join(location.Path, "catalog", "shared.txt"), "shared")
	for name, content := range map[string]string{"existing/SKILL.md": "existing", ".system/builtin/SKILL.md": "builtin"} {
		file := filepath.Join(location.Path, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	items, err := s.ScanSkills(location.ID)
	if err != nil || len(items) != 3 {
		t.Fatalf("已有技能扫描：%+v %v", items, err)
	}
	for _, item := range items {
		if item.Name == "catalog" && (len(item.Skills) != 2 || item.Mode != "repository") {
			t.Fatalf("整仓库应作为单项：%+v", item)
		}
		if item.Name == "existing" && item.Repository != "" {
			t.Fatal("不应猜测已有技能的来源")
		}
		if item.Name == ".system" && !item.ReadOnly {
			t.Fatal("系统技能必须只读")
		}
	}
	if _, err := s.DeleteSkill(location.ID, ".system"); err == nil {
		t.Fatal("不能删除系统技能")
	}
	if _, err := s.DeleteSkill(location.ID, "../existing"); err == nil {
		t.Fatal("不能越界删除")
	}
	if _, err := s.DeleteSkill(location.ID, "existing"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DeleteSkill(location.ID, "catalog"); err != nil {
		t.Fatal(err)
	}
	if len(s.store.skillsState().Bindings) != 0 {
		t.Fatal("删除后仓库绑定应同步移除")
	}
	skillTestRead(t, filepath.Join(location.Path, ".system", "builtin", "SKILL.md"), "builtin")
}

func TestSkillsUpdateProtectsLocalChangesAndMissingSources(t *testing.T) {
	s, remote, location := skillTestService(t)
	installed := skillTestInstall(t, s, location, true, "skills/alpha")[0]
	id := skillPathID(installed.Path)
	skillFile := filepath.Join(installed.Path, "SKILL.md")
	if err := os.WriteFile(skillFile, []byte("local"), 0644); err != nil {
		t.Fatal(err)
	}
	remote.mu.Lock()
	remote.revision = strings.Repeat("b", 40)
	remote.files = map[string]string{"skills/alpha/SKILL.md": "alpha v2", "skills/new/SKILL.md": "new"}
	remote.mu.Unlock()
	result, err := s.UpdateSkill(id, "")
	if err != nil || !result.NeedsConfirmation || !slices.Contains(result.Changes, "SKILL.md") {
		t.Fatalf("本地改动需确认：%+v %v", result, err)
	}
	skillTestRead(t, skillFile, "local")
	if err := os.WriteFile(skillFile, []byte("local again"), 0644); err != nil {
		t.Fatal(err)
	}
	next, err := s.UpdateSkill(id, result.Confirmation)
	if err != nil || !next.NeedsConfirmation || next.Confirmation == result.Confirmation {
		t.Fatalf("过期确认不能覆盖新修改：%+v %v", next, err)
	}
	result, err = s.UpdateSkill(id, next.Confirmation)
	if err != nil || result.NeedsConfirmation {
		t.Fatalf("覆盖更新：%+v %v", result, err)
	}
	skillTestRead(t, skillFile, "alpha v2")
	if _, err := os.Stat(filepath.Join(installed.Path, "scripts", "run.py")); !os.IsNotExist(err) {
		t.Fatal("更新应清理仓库已删除的文件")
	}
	if _, err := os.Stat(filepath.Join(location.Path, "new")); !os.IsNotExist(err) {
		t.Fatal("更新不应安装新增的其他技能")
	}
	if err := os.RemoveAll(installed.Path); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpdateSkill(id, ""); err != nil {
		t.Fatalf("缺失目录应可恢复：%v", err)
	}
	skillTestRead(t, skillFile, "alpha v2")
	remote.mu.Lock()
	remote.revision = strings.Repeat("c", 40)
	remote.files = map[string]string{"skills/new/SKILL.md": "new"}
	remote.mu.Unlock()
	if _, err := s.UpdateSkill(id, ""); err == nil {
		t.Fatal("来源技能消失应报错")
	}
	skillTestRead(t, skillFile, "alpha v2")
	remote.mu.Lock()
	remote.fail = true
	remote.mu.Unlock()
	if _, err := s.UpdateSkill(id, ""); err == nil {
		t.Fatal("网络失败应报错")
	}
	skillTestRead(t, skillFile, "alpha v2")
}

func TestSkillsPersistenceFailureRollsBackFiles(t *testing.T) {
	s, remote, location := skillTestService(t)
	installed := skillTestInstall(t, s, location, true, "skills/alpha")[0]
	remote.mu.Lock()
	remote.revision = strings.Repeat("b", 40)
	remote.files["skills/alpha/SKILL.md"] = "alpha v2"
	remote.mu.Unlock()
	dataFile := filepath.Join(s.store.root, "Trylet.json")
	if err := os.Rename(dataFile, dataFile+".previous"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(dataFile, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpdateSkill(skillPathID(installed.Path), ""); err == nil {
		t.Fatal("绑定持久化失败必须报错")
	}
	skillTestRead(t, filepath.Join(installed.Path, "SKILL.md"), "alpha v1")
	if got := s.store.skillsState().Bindings[skillPathID(installed.Path)]; got.Revision != strings.Repeat("a", 40) {
		t.Fatal("绑定持久化失败应回滚内存状态")
	}
	if _, err := s.DeleteSkill(location.ID, "alpha"); err == nil {
		t.Fatal("删除绑定保存失败必须报错")
	}
	skillTestRead(t, filepath.Join(installed.Path, "SKILL.md"), "alpha v1")
}

func TestSkillsDeleteFailureKeepsPathAndBinding(t *testing.T) {
	s, _, location := skillTestService(t)
	installed := skillTestInstall(t, s, location, true, "skills/alpha")[0]
	path := filepath.Join(installed.Path, "SKILL.md")
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(handle)
	if _, err := s.DeleteSkill(location.ID, "alpha"); err == nil {
		t.Fatal("被占用文件删除失败必须报错")
	} else {
		t.Logf("Windows 文件占用错误：%v", err)
	}
	skillTestRead(t, path, "alpha v1")
	if _, bound := s.store.skillsState().Bindings[skillPathID(installed.Path)]; !bound {
		t.Fatal("删除失败必须保留来源绑定")
	}
}

func TestSkillsSamePathRemainsVisibleInBothScopes(t *testing.T) {
	s, _, _ := skillTestService(t)
	project := filepath.Join(s.store.root, "project")
	shared := filepath.Join(project, ".agents", "skills")
	workspace, err := s.SaveSkillPreferences(SkillPreferences{Scope: "project", Project: project, Location: shared, CustomLocations: []SkillCustomLocation{{Path: shared}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, scope := range []string{"user", "project"} {
		found := false
		for _, location := range workspace.Locations {
			if location.Scope == scope && location.Path == shared {
				found = true
			}
		}
		if !found {
			t.Fatalf("同一路径在 %s 作用域中丢失", scope)
		}
	}
}

func TestSkillsUnboundDeleteFailureReportsRemainingPath(t *testing.T) {
	s, _, location := skillTestService(t)
	dir := filepath.Join(location.Path, "existing")
	scripts := filepath.Join(dir, "scripts")
	if err := os.MkdirAll(scripts, 0755); err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string]string{filepath.Join(dir, "SKILL.md"): "existing", filepath.Join(scripts, "run.py"): "keep"} {
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	name, err := windows.UTF16PtrFromString(filepath.Join(scripts, "run.py"))
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(handle)
	if _, err := s.DeleteSkill(location.ID, "existing"); err == nil || !strings.Contains(err.Error(), dir) {
		t.Fatalf("未绑定技能删除失败应提供残余原路径：%v", err)
	}
	skillTestRead(t, filepath.Join(scripts, "run.py"), "keep")
}

func TestSkillsPrivateRepositoryUsesExistingToken(t *testing.T) {
	s, remote, location := skillTestService(t)
	remote.private = true
	if _, err := s.PrepareSkillInstall("acme/catalog", true); err == nil || !strings.Contains(err.Error(), "私有仓库") {
		t.Fatalf("未认证 404 应保留鉴权可能：%v", err)
	}
	s.publisher.token = func() (string, error) { return "local-test-token", nil }
	skillTestInstall(t, s, location, true, "skills/alpha")
}

func TestSkillArchiveRejectsUnsafePathsAndLinks(t *testing.T) {
	cases := []struct {
		name  string
		paths []string
		link  bool
	}{
		{"parent", []string{"repo/../escape/SKILL.md"}, false},
		{"absolute", []string{"/repo/escape/SKILL.md"}, false},
		{"backslash", []string{`repo/skills\escape/SKILL.md`}, false},
		{"device", []string{"repo/CON/SKILL.md"}, false},
		{"trailing-dot", []string{"repo/alpha./SKILL.md"}, false},
		{"case-collision", []string{"repo/Alpha/SKILL.md", "repo/alpha/script.py"}, false},
		{"duplicate", []string{"repo/alpha/SKILL.md", "repo/alpha/SKILL.md"}, false},
		{"link", []string{"repo/alpha/SKILL.md"}, true},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			var buffer bytes.Buffer
			z := zip.NewWriter(&buffer)
			for _, path := range test.paths {
				header := &zip.FileHeader{Name: path}
				if test.link {
					header.SetMode(os.ModeSymlink | 0777)
				}
				file, err := z.CreateHeader(header)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := file.Write([]byte("content")); err != nil {
					t.Fatal(err)
				}
			}
			if err := z.Close(); err != nil {
				t.Fatal(err)
			}
			archive := filepath.Join(root, "archive.zip")
			if err := os.WriteFile(archive, buffer.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
			if err := extractSkillArchive(archive, filepath.Join(root, "output")); err == nil {
				t.Fatal("应拒绝不安全压缩包")
			}
			if _, err := os.Stat(filepath.Join(root, "output")); !os.IsNotExist(err) {
				t.Fatal("校验失败不应写入解压目录")
			}
		})
	}
}

func TestSkillsDeleteJunctionKeepsOriginal(t *testing.T) {
	s, _, location := skillTestService(t)
	original := filepath.Join(s.store.root, "original")
	if err := os.MkdirAll(original, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(original, "SKILL.md"), []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(location.Path, 0755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(location.Path, "linked")
	if output, err := hiddenCommand(context.Background(), "cmd", "/c", "mklink", "/J", link, original).CombinedOutput(); err != nil {
		t.Fatalf("创建测试 junction：%v %s", err, output)
	}
	items, err := s.ScanSkills(location.ID)
	if err != nil || len(items) != 1 || !items[0].Linked {
		t.Fatalf("应标记链接目录：%+v %v", items, err)
	}
	if _, err := s.DeleteSkill(location.ID, "linked"); err != nil {
		t.Fatal(err)
	}
	skillTestRead(t, filepath.Join(original, "SKILL.md"), "keep")
	if _, err := os.Lstat(link); !os.IsNotExist(err) {
		t.Fatal("链接应被移除")
	}
}

func TestSkillsOldDataAndRepositoryValidation(t *testing.T) {
	root := t.TempDir()
	legacy := []byte(`{"version":1,"current":"","settings":{},"drafts":{}}`)
	if err := os.WriteFile(filepath.Join(root, "Trylet.json"), legacy, 0600); err != nil {
		t.Fatal(err)
	}
	store, err := NewStore(root, root)
	if err != nil || len(store.skillsState().Bindings) != 0 {
		t.Fatalf("旧数据应兼容：%v", err)
	}
	for _, raw := range []string{"http://github.com/acme/catalog", "https://example.com/acme/catalog", "https://user:secret@github.com/acme/catalog", "https://github.com/acme/catalog/tree/main", "../catalog", "acme/CON"} {
		if _, err := skillRepository(raw); err == nil {
			t.Fatalf("应拒绝地址 %s", raw)
		}
	}
}

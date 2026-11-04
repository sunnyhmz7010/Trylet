package skills

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	githubapi "trylet/internal/github"
	"trylet/internal/identity"
)

func TestSkillDownloadCancellationDoesNotBlockWorkspace(t *testing.T) {
	for _, phase := range []string{"connecting", "downloading"} {
		t.Run(phase, func(t *testing.T) {
			s, _, location := skillTestService(t)
			started := make(chan struct{})
			server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if phase == "connecting" || strings.Contains(r.URL.Path, "/zipball/") {
					if phase == "downloading" {
						w.WriteHeader(http.StatusOK)
						fmt.Fprint(w, "partial archive")
						w.(http.Flusher).Flush()
					}
					close(started)
					<-r.Context().Done()
					return
				}
				if strings.Contains(r.URL.Path, "/commits/") {
					fmt.Fprintf(w, `{"sha":%q}`, strings.Repeat("a", 40))
				} else {
					fmt.Fprint(w, `{"default_branch":"main"}`)
				}
			}))
			var err error
			server.Listener, err = net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			server.Start()
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(func() { cancel(); server.Close() })
			s.publisher = githubapi.NewPublisher(s.store.ReadToken, server.URL)
			id := identity.New()
			done := make(chan error, 1)
			go func() {
				_, err := s.PrepareSkillInstall(ctx, id, "acme/catalog", true)
				done <- err
			}()
			select {
			case <-started:
			case <-time.After(3 * time.Second):
				t.Fatal("下载未进入可取消的网络阶段")
			}
			workspace := make(chan error, 1)
			go func() {
				_, err := s.GetSkillWorkspace()
				if err == nil {
					_, err = s.ScanSkills(location.ID)
				}
				workspace <- err
			}()
			select {
			case err := <-workspace:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("网络下载阻塞了 Skills 工作区")
			}
			s.DiscardSkillPreview(id)
			select {
			case err := <-done:
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("取消未传递到网络请求：%v", err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("取消后下载没有及时结束")
			}
			if len(s.skillDownloads) != 0 {
				t.Fatal("取消后仍占用下载预览名额")
			}
			entries, err := os.ReadDir(s.store.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), "skills-") {
					t.Fatal("取消后遗留了下载文件")
				}
			}
		})
	}
}

func TestSkillDownloadCancellationDuringLocalPreparation(t *testing.T) {
	for _, phase := range []string{"extracting", "scanning"} {
		t.Run(phase, func(t *testing.T) {
			s, _, _ := skillTestService(t)
			id := identity.New()
			s.skillProgress = func(progress SkillDownloadProgress) {
				if progress.ID == id && progress.Phase == phase {
					s.DiscardSkillPreview(id)
				}
			}
			if _, err := s.PrepareSkillInstall(context.Background(), id, "acme/catalog", true); !errors.Is(err, context.Canceled) {
				t.Fatalf("本地处理阶段未取消：%v", err)
			}
			if len(s.skillDownloads) != 0 {
				t.Fatal("取消后遗留了预览")
			}
			entries, err := os.ReadDir(s.store.TempDir())
			if err != nil || len(entries) != 0 {
				t.Fatalf("取消后遗留了临时文件：%v, %v", entries, err)
			}
		})
	}
}

func TestSkillUpdateDoesNotRestoreDeletedInstallation(t *testing.T) {
	s, remote, location := skillTestService(t)
	installed := skillTestInstall(t, s, location, true, "skills/alpha")[0]
	started, release := make(chan struct{}), make(chan struct{})
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
	})
	remote.mu.Lock()
	remote.revision = strings.Repeat("b", 40)
	remote.zipStarted, remote.zipRelease = started, release
	remote.mu.Unlock()
	done := make(chan error, 1)
	go func() {
		_, err := s.UpdateSkill(skillPathID(installed.Path), "")
		done <- err
	}()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("更新没有进入下载阶段")
	}
	deleted := make(chan error, 1)
	go func() {
		_, err := s.DeleteSkill(location.ID, "alpha")
		deleted <- err
	}()
	select {
	case err := <-deleted:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("更新下载阻塞了其他 Skills 操作")
	}
	close(release)
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "安装记录") {
			t.Fatalf("更新没有检测到下载期间的删除：%v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("更新未结束")
	}
	if _, err := os.Stat(filepath.Join(location.Path, "alpha")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("更新重新创建了已删除的安装目录")
	}
	if _, exists := s.store.SkillsState().Bindings[skillPathID(installed.Path)]; exists {
		t.Fatal("更新重新创建了已删除的仓库绑定")
	}
}

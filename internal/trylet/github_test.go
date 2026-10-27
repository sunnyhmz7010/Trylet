package trylet

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

type testGitHub struct {
	mu               sync.Mutex
	code, sha        string
	puts             int
	deny, concurrent bool
}

func (g *testGitHub) handler(w http.ResponseWriter, r *http.Request) {
	g.mu.Lock()
	defer g.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	if r.Header.Get("Authorization") != "Bearer local-test-token" {
		w.WriteHeader(401)
		fmt.Fprint(w, `{"message":"bad token"}`)
		return
	}
	if strings.Contains(r.URL.Path, "/contents/") {
		if r.Method == "GET" {
			if g.sha == "" {
				w.WriteHeader(404)
				fmt.Fprint(w, `{"message":"Not Found"}`)
				return
			}
			_ = json.NewEncoder(w).Encode(remoteFile{SHA: g.sha, Type: "file", Encoding: "base64", Content: base64.StdEncoding.EncodeToString([]byte(g.code)), HTMLURL: "https://github.com/owner/repo/blob/main/test.py"})
			return
		}
		if r.Method == "PUT" {
			g.puts++
			if g.deny {
				w.WriteHeader(403)
				fmt.Fprint(w, `{"message":"protected branch"}`)
				return
			}
			var body struct{ Content, SHA, Branch string }
			_ = json.NewDecoder(r.Body).Decode(&body)
			if g.concurrent {
				g.code = "remote change"
				g.sha = "changed"
				g.concurrent = false
			}
			if body.SHA != g.sha {
				w.WriteHeader(409)
				fmt.Fprint(w, `{"message":"sha changed"}`)
				return
			}
			b, _ := base64.StdEncoding.DecodeString(body.Content)
			g.code = string(b)
			g.sha = fmt.Sprintf("sha-%d", g.puts)
			_ = json.NewEncoder(w).Encode(map[string]any{"content": remoteFile{SHA: g.sha, HTMLURL: "https://github.com/owner/repo/blob/main/test.py"}, "commit": map[string]string{"html_url": "https://github.com/owner/repo/commit/test"}})
			return
		}
	}
	if strings.Contains(r.URL.Path, "/git/trees/") {
		ref := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
		entries := []treeEntry{}
		if ref == "main" {
			entries = []treeEntry{{Path: "主题", Type: "tree", SHA: "topic-tree"}, {Path: "README.md", Type: "blob", SHA: "readme"}}
		}
		_ = json.NewEncoder(w).Encode(gitTree{Tree: entries})
		return
	}
	if strings.HasSuffix(r.URL.Path, "/branches") {
		page := r.URL.Query().Get("page")
		batch := []map[string]string{}
		if page == "1" {
			for i := 0; i < 100; i++ {
				batch = append(batch, map[string]string{"name": fmt.Sprintf("branch-%d", i)})
			}
		} else if page == "2" {
			batch = append(batch, map[string]string{"name": "main"})
		}
		_ = json.NewEncoder(w).Encode(batch)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"default_branch": "main", "size": 1})
}

func fakePublisher(t *testing.T, g *testGitHub) *Publisher {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(g.handler))
	t.Cleanup(server.Close)
	p := NewPublisher(func() (string, error) { return "local-test-token", nil })
	p.base = server.URL
	return p
}
func publishFixture() PublishRequest {
	return PublishRequest{DraftID: newID(), Code: "print('你好')\n", Language: "python", Target: PublishTarget{Repository: "owner/repo", Branch: "main", Path: "2026-10-27/练习.py"}, Message: "feat(practice): 添加练习"}
}

func TestPublishCreateIdenticalAndConflict(t *testing.T) {
	g := &testGitHub{}
	p := fakePublisher(t, g)
	req := publishFixture()
	ctx := context.Background()
	created, err := p.Publish(ctx, req)
	if err != nil || created.Status != "published" || g.code != req.Code || g.puts != 1 {
		t.Fatalf("创建失败：%+v %v", created, err)
	}
	identical, err := p.Publish(ctx, req)
	if err != nil || identical.Status != "synced" || g.puts != 1 {
		t.Fatalf("重复提交未去重：%+v %v", identical, err)
	}
	req.Code = "print(2)\n"
	conflict, err := p.Publish(ctx, req)
	if err != nil || conflict.Status != "conflict" || conflict.RemoteCode != "print('你好')\n" || g.puts != 1 {
		t.Fatalf("未阻止覆盖：%+v %v", conflict, err)
	}
	req.Confirmed = true
	req.ExpectedSHA = "old"
	stale, err := p.Publish(ctx, req)
	if err != nil || stale.Status != "conflict" || g.puts != 1 {
		t.Fatal("使用旧 SHA 覆盖了文件")
	}
	req.ExpectedSHA = conflict.SHA
	updated, err := p.Publish(ctx, req)
	if err != nil || updated.Status != "published" || g.code != req.Code || g.puts != 2 {
		t.Fatalf("确认更新失败：%+v %v", updated, err)
	}
}

func TestPublishConcurrentChangeAndPermission(t *testing.T) {
	g := &testGitHub{code: "old", sha: "base", concurrent: true}
	p := fakePublisher(t, g)
	req := publishFixture()
	req.Confirmed = true
	req.ExpectedSHA = "base"
	result, err := p.Publish(context.Background(), req)
	if err != nil || result.Status != "conflict" || result.SHA != "changed" || result.RemoteCode != "remote change" {
		t.Fatalf("并发修改未回到冲突选择：%+v %v", result, err)
	}
	g.deny = true
	req.ExpectedSHA = "changed"
	_, err = p.Publish(context.Background(), req)
	var ae *apiError
	if !errors.As(err, &ae) || ae.status != 403 || g.code != "remote change" {
		t.Fatalf("权限错误处理不正确：%v", err)
	}
}

type lostResponse struct{ base http.RoundTripper }

func (l lostResponse) RoundTrip(req *http.Request) (*http.Response, error) {
	response, err := l.base.RoundTrip(req)
	if err == nil && req.Method == "PUT" {
		_, _ = io.Copy(io.Discard, response.Body)
		_ = response.Body.Close()
		return nil, io.ErrUnexpectedEOF
	}
	return response, err
}

func TestPublishLostResponseReconcilesWithoutDuplicate(t *testing.T) {
	g := &testGitHub{}
	p := fakePublisher(t, g)
	p.client = &http.Client{Transport: lostResponse{http.DefaultTransport}}
	result, err := p.Publish(context.Background(), publishFixture())
	if err != nil || result.Status != "synced" || g.puts != 1 {
		t.Fatalf("响应丢失后未核对远端：%+v %v puts=%d", result, err, g.puts)
	}
}

func TestGitHubDirectoriesAndAllBranches(t *testing.T) {
	g := &testGitHub{}
	p := fakePublisher(t, g)
	ctx := context.Background()
	info, err := p.Repository(ctx, "owner/repo")
	if err != nil || len(info.Branches) != 101 || info.Branches[100] != "main" {
		t.Fatalf("分支集合不完整：%+v %v", info, err)
	}
	target := PublishTarget{Repository: "owner/repo", Branch: "main"}
	root, err := p.Directory(ctx, target, "")
	if err != nil || len(root.Names) != 1 || root.Names[0] != "主题" {
		t.Fatalf("根目录失败：%+v %v", root, err)
	}
	child, err := p.Directory(ctx, target, "主题")
	if err != nil || !child.Exists || len(child.Names) != 0 {
		t.Fatalf("中文目录失败：%+v %v", child, err)
	}
	newDir, err := p.Directory(ctx, target, "2026-10-27/基础")
	if err != nil || newDir.Exists {
		t.Fatalf("新目录建议失败：%+v %v", newDir, err)
	}
}

func TestPublishInvalidPathDoesNotWrite(t *testing.T) {
	g := &testGitHub{}
	p := fakePublisher(t, g)
	req := publishFixture()
	for _, v := range []string{"../test.py", "/test.py", "test.cpp", "a\\test.py"} {
		req.Target.Path = v
		if _, err := p.Publish(context.Background(), req); err == nil {
			t.Errorf("接受了无效路径 %q", v)
		}
	}
	if g.puts != 0 {
		t.Fatal("无效输入触发了远端写入")
	}
}

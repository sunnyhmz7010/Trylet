package app

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	githubapi "trylet/internal/github"
	publication "trylet/internal/publish"
	storage "trylet/internal/store"
)

func TestPublishUsesInjectedClientAndPersistsRecord(t *testing.T) {
	root := t.TempDir()
	store, err := storage.NewStore(root, root)
	if err != nil {
		t.Fatal(err)
	}
	draft, err := store.Current()
	if err != nil {
		t.Fatal(err)
	}
	requests := 0
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer local-test-token" || r.URL.Path != "/repos/owner/repo/contents/example.py" {
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		requests++
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"message":"Not Found"}`)
			return
		}
		var body struct{ Content string }
		if r.Method != http.MethodPut || json.NewDecoder(r.Body).Decode(&body) != nil || body.Content != base64.StdEncoding.EncodeToString([]byte("print(42)")) {
			http.Error(w, "unexpected publication", http.StatusBadRequest)
			return
		}
		fmt.Fprint(w, `{"content":{"sha":"saved-sha","html_url":"https://github.com/owner/repo/blob/main/example.py"},"commit":{"html_url":"https://github.com/owner/repo/commit/saved"}}`)
	}))
	server.Listener, err = net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server.Start()
	t.Cleanup(server.Close)
	publisher := githubapi.NewPublisher(func() (string, error) { return "local-test-token", nil }, server.URL)
	service := NewService(store, nil, publisher, nil)
	result, err := service.Publish(githubapi.PublishRequest{DraftID: draft.ID, Code: "print(42)", Language: "python", Message: "test: 发布练习", Target: publication.PublishTarget{Repository: "owner/repo", Branch: "main", Path: "example.py"}})
	if err != nil || result.Status != "published" || result.Record == nil || requests != 2 {
		t.Fatalf("注入的客户端未完成发布：%+v，%v，请求数 %d", result, err, requests)
	}
	saved, err := store.Load(draft.ID)
	if err != nil || saved.Publish == nil || *saved.Publish != *result.Record {
		t.Fatalf("发布结果未保存到草稿：%+v，%v", saved, err)
	}
}

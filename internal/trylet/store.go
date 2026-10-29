package trylet

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/windows"
)

type diskState struct {
	Version        int               `json:"version"`
	Current        string            `json:"current"`
	Settings       Settings          `json:"settings"`
	Drafts         map[string]Draft  `json:"drafts"`
	Token          []byte            `json:"encryptedToken,omitempty"`
	GitRecent      []GitRecent       `json:"gitRecent,omitempty"`
	GitPreferences map[string]string `json:"gitPreferences,omitempty"`
	Skills         skillState        `json:"skills,omitempty"`
}

type Store struct {
	mu      sync.Mutex
	root    string
	tempDir string
	state   diskState
}

func NewStore(root, tempDir string) (*Store, error) {
	s := &Store{root: root, tempDir: tempDir, state: diskState{Version: 1, Settings: Settings{AutoTemplate: true, Shortcut: "Ctrl+Alt+Space", Runtimes: map[string]string{}, CustomPaths: map[string]string{}}, Drafts: map[string]Draft{}}}
	err := readJSON(filepath.Join(root, "Trylet.json"), &s.state)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("读取同级数据文件失败，原文件已保留：%w", err)
	}
	if s.state.Version != 1 {
		return nil, errors.New("数据文件版本不受支持，原文件已保留")
	}
	if s.state.Drafts == nil {
		s.state.Drafts = map[string]Draft{}
	}
	if s.state.Settings.Runtimes == nil {
		s.state.Settings.Runtimes = map[string]string{}
	}
	if s.state.Settings.CustomPaths == nil {
		s.state.Settings.CustomPaths = map[string]string{}
	}
	return s, nil
}

func newID() string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}

func validID(id string) bool { b, err := hex.DecodeString(id); return err == nil && len(b) == 12 }

func atomicJSON(path, tempDir string, value any) error {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(path, tempDir, append(b, '\n'))
}

func atomicWrite(path, tempDir string, data []byte) error {
	f, err := os.CreateTemp(tempDir, ".Trylet-save-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	_, writeErr := f.Write(data)
	syncErr := f.Sync()
	closeErr := f.Close()
	if err = errors.Join(writeErr, syncErr, closeErr); err != nil {
		return err
	}
	from, err := windows.UTF16PtrFromString(f.Name())
	if err != nil {
		return err
	}
	to, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	return windows.MoveFileEx(from, to, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH)
}

func readJSON(path string, value any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, value)
}

func (s *Store) load(id string) (Draft, error) {
	var d Draft
	if !validID(id) {
		return d, errors.New("草稿标识无效")
	}
	d, ok := s.state.Drafts[id]
	if !ok {
		return Draft{}, errors.New("草稿不存在")
	}
	if d.Publish != nil {
		publication := *d.Publish
		d.Publish = &publication
	}
	return d, nil
}

func (s *Store) persist(d Draft) error {
	old, existed := s.state.Drafts[d.ID]
	s.state.Drafts[d.ID] = d
	if err := s.save(); err != nil {
		if existed {
			s.state.Drafts[d.ID] = old
		} else {
			delete(s.state.Drafts, d.ID)
		}
		return err
	}
	return nil
}

func (s *Store) save() error {
	return atomicJSON(filepath.Join(s.root, "Trylet.json"), s.tempDir, s.state)
}

func (s *Store) Current() (Draft, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state.Current != "" {
		return s.load(s.state.Current)
	}
	return s.create()
}

func (s *Store) create() (Draft, error) {
	d := Draft{ID: newID(), Title: "未命名练习", Language: "auto", UpdatedAt: time.Now().Format(time.RFC3339Nano)}
	previous := s.state.Current
	s.state.Current = d.ID
	if err := s.persist(d); err != nil {
		s.state.Current = previous
		return Draft{}, err
	}
	return d, nil
}

func (s *Store) NewDraft() (Draft, error) { s.mu.Lock(); defer s.mu.Unlock(); return s.create() }

func (s *Store) LoadDraft(id string) (Draft, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.load(id)
	if err != nil {
		return d, err
	}
	previous := s.state.Current
	s.state.Current = id
	if err := s.save(); err != nil {
		s.state.Current = previous
		return Draft{}, err
	}
	return d, nil
}

func (s *Store) SaveDraft(in DraftInput) (Draft, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.load(in.ID)
	if err != nil {
		return d, err
	}
	if in.Language != "auto" && extension(in.Language) == "" {
		return d, errors.New("不支持该语言")
	}
	if len(in.Code) > 1024*1024 {
		return d, errors.New("单份练习最多支持 1 MiB 源码")
	}
	d.Code = strings.ReplaceAll(in.Code, "\r\n", "\n")
	d.Language = in.Language
	d.Title = strings.TrimSpace(in.Title)
	if d.Title == "" {
		d.Title = "未命名练习"
	}
	d.UpdatedAt = time.Now().Format(time.RFC3339Nano)
	return d, s.persist(d)
}

func (s *Store) SavePublication(id string, record PublishRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.load(id)
	if err != nil {
		return err
	}
	d.Publish = &record
	if d.Title == "未命名练习" {
		d.Title = filepath.Base(record.Target.Path)
	}
	return s.persist(d)
}

func (s *Store) History() []DraftSummary {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]DraftSummary, 0, len(s.state.Drafts))
	for _, d := range s.state.Drafts {
		result = append(result, DraftSummary{ID: d.ID, Title: d.Title, Language: d.Language, UpdatedAt: d.UpdatedAt})
	}
	// RFC3339Nano 的小数位数可变，同一秒内不能直接按字符串排序。
	sort.Slice(result, func(i, j int) bool {
		left, leftErr := time.Parse(time.RFC3339Nano, result[i].UpdatedAt)
		right, rightErr := time.Parse(time.RFC3339Nano, result[j].UpdatedAt)
		if (leftErr == nil) != (rightErr == nil) {
			return leftErr == nil
		}
		if leftErr != nil {
			return result[i].UpdatedAt > result[j].UpdatedAt
		}
		return left.After(right)
	})
	return result
}

func (s *Store) Settings() Settings {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, _ := json.Marshal(s.state.Settings)
	var v Settings
	_ = json.Unmarshal(b, &v)
	return v
}

func (s *Store) SaveSettings(v Settings) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	old := s.state.Settings
	v.BallX = old.BallX
	v.BallY = old.BallY
	v.BallPlaced = old.BallPlaced
	s.state.Settings = v
	if err := s.save(); err != nil {
		s.state.Settings = old
		return err
	}
	return nil
}

func (s *Store) SaveBallPosition(x, y int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	old := s.state.Settings
	s.state.Settings.BallX = x
	s.state.Settings.BallY = y
	s.state.Settings.BallPlaced = true
	if err := s.save(); err != nil {
		s.state.Settings = old
		return err
	}
	return nil
}

func extension(language string) string {
	switch language {
	case "c":
		return ".c"
	case "cpp":
		return ".cpp"
	case "python":
		return ".py"
	}
	return ""
}

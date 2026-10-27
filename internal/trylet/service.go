package trylet

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
)

type Service struct {
	store            *Store
	runner           *Runner
	publisher        *Publisher
	mu               sync.Mutex
	candidates       []RuntimeCandidate
	detectGeneration uint64
	settingsChanged  func(Settings) error
	notice           string
}

func NewService(store *Store, runner *Runner, settingsChanged func(Settings) error) *Service {
	s := &Service{store: store, runner: runner, settingsChanged: settingsChanged}
	s.publisher = NewPublisher(store.readToken)
	return s
}

func (s *Service) SetToken(token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return errors.New("访问令牌不能为空")
	}
	if len(token) > 2400 {
		return errors.New("访问令牌长度无效")
	}
	return s.store.saveToken(token)
}

func (s *Service) RemoveToken() error {
	return s.store.saveToken("")
}

func (s *Service) Bootstrap() (Bootstrap, error) {
	d, err := s.store.Current()
	if err != nil {
		return Bootstrap{}, err
	}
	s.mu.Lock()
	notice := s.notice
	s.mu.Unlock()
	return Bootstrap{Draft: d, History: s.store.History(), Settings: s.store.Settings(), TokenSet: s.store.hasToken(), Notice: notice}, nil
}

func (s *Service) SaveDraft(in DraftInput) (Draft, error) { return s.store.SaveDraft(in) }
func (s *Service) NewDraft() (Draft, error) {
	if s.runner.ActiveID() != "" {
		return Draft{}, errors.New("请先停止当前运行")
	}
	return s.store.NewDraft()
}
func (s *Service) LoadDraft(id string) (Draft, error) {
	if s.runner.ActiveID() != "" {
		return Draft{}, errors.New("请先停止当前运行")
	}
	return s.store.LoadDraft(id)
}
func (s *Service) History() []DraftSummary { return s.store.History() }

func (s *Service) SaveSettings(v Settings) error {
	if v.Runtimes == nil {
		v.Runtimes = map[string]string{}
	}
	if v.CustomPaths == nil {
		v.CustomPaths = map[string]string{}
	}
	for lang, p := range v.CustomPaths {
		if extension(lang) == "" {
			return errors.New("自定义环境语言无效")
		}
		if p != "" {
			info, err := os.Stat(p)
			if err != nil || info.IsDir() {
				return fmt.Errorf("%s 环境路径不存在，请选择可执行文件", lang)
			}
		}
	}
	if v.Repository != "" {
		if _, err := repositoryEndpoint(v.Repository); err != nil {
			return err
		}
	}
	old := s.store.Settings()
	if s.settingsChanged != nil {
		if err := s.settingsChanged(v); err != nil {
			return err
		}
	}
	if err := s.store.SaveSettings(v); err != nil {
		if s.settingsChanged != nil {
			if rollbackErr := s.settingsChanged(old); rollbackErr != nil {
				return errors.Join(err, rollbackErr)
			}
		}
		return err
	}
	s.mu.Lock()
	s.detectGeneration++
	s.mu.Unlock()
	return nil
}

func (s *Service) SavePreferences(v Settings, token string) (bool, error) {
	old := s.store.Settings()
	if err := s.SaveSettings(v); err != nil {
		return false, err
	}
	if strings.TrimSpace(token) != "" {
		if err := s.SetToken(token); err != nil {
			return false, errors.Join(err, s.SaveSettings(old))
		}
	}
	return s.store.hasToken(), nil
}

func (s *Service) DetectRuntimes() []RuntimeCandidate {
	s.mu.Lock()
	s.detectGeneration++
	generation := s.detectGeneration
	s.mu.Unlock()
	v := discoverRuntimes(s.store.Settings())
	s.mu.Lock()
	if generation == s.detectGeneration {
		s.candidates = v
	}
	result := append([]RuntimeCandidate{}, s.candidates...)
	s.mu.Unlock()
	return result
}

func (s *Service) StartRun(req RunRequest) (string, error) {
	if _, err := s.store.loadForRun(req.DraftID); err != nil {
		return "", err
	}
	s.mu.Lock()
	var selected *RuntimeCandidate
	for _, candidate := range s.candidates {
		if candidate.ID == req.RuntimeID && candidate.Language == req.Language {
			v := candidate
			selected = &v
			break
		}
	}
	s.mu.Unlock()
	if selected == nil {
		return "", errors.New("请选择已检测到的可用环境，或在设置中重新检测")
	}
	return s.runner.Start(req, *selected)
}
func (s *Service) WriteInput(id, data string) error { return s.runner.Write(id, data) }
func (s *Service) ResizeTerminal(id string, cols, rows int) error {
	return s.runner.Resize(id, cols, rows)
}
func (s *Service) StopRun(id string) error { return s.runner.Stop(id) }
func (s *Service) Repository(repository string) (RepositoryInfo, error) {
	return s.publisher.Repository(context.Background(), repository)
}
func (s *Service) Directory(target PublishTarget, dir string) (Directory, error) {
	return s.publisher.Directory(context.Background(), target, dir)
}
func (s *Service) Publish(req PublishRequest) (PublishResult, error) {
	if _, err := s.store.loadForRun(req.DraftID); err != nil {
		return PublishResult{}, err
	}
	result, err := s.publisher.Publish(context.Background(), req)
	if err != nil {
		return result, err
	}
	if result.Record != nil {
		if err = s.store.SavePublication(req.DraftID, *result.Record); err != nil {
			return result, fmt.Errorf("远端已同步，但本地发布记录保存失败：%w", err)
		}
	}
	return result, nil
}

func (s *Store) loadForRun(id string) (Draft, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.load(id)
}

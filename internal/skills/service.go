package skills

import (
	"sync"

	githubapi "trylet/internal/github"
	storage "trylet/internal/store"
)

type Service struct {
	store          *storage.Store
	publisher      *githubapi.Publisher
	skillsMu       sync.Mutex
	skillHome      string
	skillDownloads map[string]*skillDownload
	skillProgress  func(SkillDownloadProgress)
}

func NewService(store *storage.Store, publisher *githubapi.Publisher, progress func(SkillDownloadProgress)) *Service {
	return &Service{store: store, publisher: publisher, skillProgress: progress}
}

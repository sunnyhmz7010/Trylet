package desktop

import (
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	gitrepo "trylet/internal/git"
	execution "trylet/internal/runner"
	storage "trylet/internal/store"
)

type Desktop struct {
	mu             sync.Mutex
	app            *application.App
	ball           *application.WebviewWindow
	editor         *application.WebviewWindow
	store          *storage.Store
	runner         *execution.Runner
	git            *gitrepo.GitReader
	shortcut       string
	notice         string
	ready          bool
	showRequested  bool
	closeRequested bool
	moveTimer      *time.Timer
	ballWidth      int
	ballReady      bool
	selectionWake  chan struct{}
	selectionStop  chan struct{}
}

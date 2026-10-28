package trylet

type Settings struct {
	Theme         string            `json:"theme"`
	GitDiffLayout string            `json:"gitDiffLayout"`
	AutoTemplate  bool              `json:"autoTemplate"`
	Shortcut      string            `json:"shortcut"`
	Runtimes      map[string]string `json:"runtimes"`
	CustomPaths   map[string]string `json:"customPaths"`
	Repository    string            `json:"repository"`
	Branch        string            `json:"branch"`
	BallX         int               `json:"ballX"`
	BallY         int               `json:"ballY"`
	BallPlaced    bool              `json:"ballPlaced"`
}

type PublishTarget struct {
	Repository string `json:"repository"`
	Branch     string `json:"branch"`
	Path       string `json:"path"`
}

type PublishRecord struct {
	Target    PublishTarget `json:"target"`
	SHA       string        `json:"sha"`
	FileURL   string        `json:"fileURL"`
	CommitURL string        `json:"commitURL"`
}

type Draft struct {
	ID        string         `json:"id"`
	Title     string         `json:"title"`
	Language  string         `json:"language"`
	Code      string         `json:"code"`
	UpdatedAt string         `json:"updatedAt"`
	Publish   *PublishRecord `json:"publish,omitempty"`
}

type DraftInput struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Language string `json:"language"`
	Code     string `json:"code"`
}

type DraftSummary struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Language  string `json:"language"`
	UpdatedAt string `json:"updatedAt"`
}

type Bootstrap struct {
	Draft    Draft          `json:"draft"`
	History  []DraftSummary `json:"history"`
	Settings Settings       `json:"settings"`
	TokenSet bool           `json:"tokenSet"`
	Notice   string         `json:"notice"`
}

type RuntimeCandidate struct {
	ID       string   `json:"id"`
	Language string   `json:"language"`
	Name     string   `json:"name"`
	Path     string   `json:"path"`
	Version  string   `json:"version"`
	Kind     string   `json:"kind"`
	Env      []string `json:"-"`
}

type RunRequest struct {
	ID        string `json:"id"`
	DraftID   string `json:"draftID"`
	Code      string `json:"code"`
	Language  string `json:"language"`
	RuntimeID string `json:"runtimeID"`
	Cols      int    `json:"cols"`
	Rows      int    `json:"rows"`
}

type RunEvent struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Data     string `json:"data"`
	Phase    string `json:"phase"`
	ExitCode int    `json:"exitCode"`
	Message  string `json:"message"`
}

type RepositoryInfo struct {
	DefaultBranch string   `json:"defaultBranch"`
	Branches      []string `json:"branches"`
}

type Directory struct {
	Path   string   `json:"path"`
	Names  []string `json:"names"`
	Exists bool     `json:"exists"`
}

type PublishRequest struct {
	DraftID     string        `json:"draftID"`
	Code        string        `json:"code"`
	Language    string        `json:"language"`
	Target      PublishTarget `json:"target"`
	Message     string        `json:"message"`
	ExpectedSHA string        `json:"expectedSHA"`
	Confirmed   bool          `json:"confirmed"`
}

type PublishResult struct {
	Status     string         `json:"status"`
	RemoteCode string         `json:"remoteCode"`
	SHA        string         `json:"sha"`
	Message    string         `json:"message"`
	Record     *PublishRecord `json:"record,omitempty"`
}

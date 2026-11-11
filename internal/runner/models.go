package runner

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

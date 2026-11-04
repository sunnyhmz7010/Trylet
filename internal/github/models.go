package github

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

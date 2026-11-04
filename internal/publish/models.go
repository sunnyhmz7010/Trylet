package publish

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

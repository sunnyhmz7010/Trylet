package github

import (
	publication "trylet/internal/publish"
)

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
	DraftID     string                    `json:"draftID"`
	Code        string                    `json:"code"`
	Language    string                    `json:"language"`
	Target      publication.PublishTarget `json:"target"`
	Message     string                    `json:"message"`
	ExpectedSHA string                    `json:"expectedSHA"`
	Confirmed   bool                      `json:"confirmed"`
}

type PublishResult struct {
	Status     string                     `json:"status"`
	RemoteCode string                     `json:"remoteCode"`
	SHA        string                     `json:"sha"`
	Message    string                     `json:"message"`
	Record     *publication.PublishRecord `json:"record,omitempty"`
}

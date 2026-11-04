package store

import (
	githubapi "trylet/internal/github"
)

type Settings struct {
	Theme               string            `json:"theme"`
	GitDiffLayout       string            `json:"gitDiffLayout"`
	AutoTemplate        bool              `json:"autoTemplate"`
	Shortcut            string            `json:"shortcut"`
	CloseAction         string            `json:"closeAction"`
	RememberCloseAction bool              `json:"rememberCloseAction"`
	BallEnabled         bool              `json:"ballEnabled"`
	BallFollowWindow    bool              `json:"ballFollowWindow"`
	Runtimes            map[string]string `json:"runtimes"`
	CustomPaths         map[string]string `json:"customPaths"`
	Repository          string            `json:"repository"`
	Branch              string            `json:"branch"`
	BallX               int               `json:"ballX"`
	BallY               int               `json:"ballY"`
	BallPlaced          bool              `json:"ballPlaced"`
}

type Draft struct {
	ID        string                   `json:"id"`
	Title     string                   `json:"title"`
	Language  string                   `json:"language"`
	Code      string                   `json:"code"`
	UpdatedAt string                   `json:"updatedAt"`
	Publish   *githubapi.PublishRecord `json:"publish,omitempty"`
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

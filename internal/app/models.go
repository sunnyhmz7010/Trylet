package app

import (
	storage "trylet/internal/store"
)

type Bootstrap struct {
	Draft    storage.Draft          `json:"draft"`
	History  []storage.DraftSummary `json:"history"`
	Settings storage.Settings       `json:"settings"`
	TokenSet bool                   `json:"tokenSet"`
	Notice   string                 `json:"notice"`
}

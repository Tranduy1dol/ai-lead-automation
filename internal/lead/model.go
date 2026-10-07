package lead

import "time"

type Lead struct {
	ID             int64
	Name           string
	Email          string
	Company        string
	Message        string
	Classification Classification
	CreatedAt      time.Time
}

type CreateParams struct {
	Name    string
	Email   string
	Company string
	Message string
}

type Classification struct {
	Category          string
	Priority          string
	Summary           string
	RecommendedAction string
}

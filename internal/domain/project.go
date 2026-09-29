package domain

import "time"

type Project struct {
	ID   string
	Name string
	// ResearchQuestion is the free-text theme a question-first project was
	// created from. Empty for projects created from a name or a file.
	ResearchQuestion string
	CreatedAt        time.Time
}

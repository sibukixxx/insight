// Package sampledata provides the fictional demo dataset used for sales
// demos. Whether the dataset is actually compiled into the binary is
// controlled entirely by the "demo" Go build tag (see embed_demo.go /
// embed_delivery.go): a binary built without that tag (the default,
// intended for customer delivery) never links the sample interview text
// in, so it cannot leak into a confidential deliverable by accident.
package sampledata

import (
	"encoding/json"
	"errors"
	"fmt"

	"insight-lab/internal/domain"
)

// ErrNotEmbedded is returned by Load when the binary was built without the
// "demo" tag, i.e. a delivery build.
var ErrNotEmbedded = errors.New("demo data is not embedded in this build")

type rawDocument struct {
	ID       string            `json:"id"`
	Source   string            `json:"source"`
	Title    string            `json:"title"`
	Content  string            `json:"content"`
	Metadata map[string]string `json:"metadata"`
}

func Load() ([]*domain.Document, error) {
	if !Embedded {
		return nil, ErrNotEmbedded
	}

	var raws []rawDocument
	if err := json.Unmarshal(payload(), &raws); err != nil {
		return nil, fmt.Errorf("parse embedded demo dataset: %w", err)
	}

	docs := make([]*domain.Document, 0, len(raws))
	for _, raw := range raws {
		docs = append(docs, &domain.Document{
			ID:       raw.ID,
			Source:   domain.SourceType(raw.Source),
			Title:    raw.Title,
			Content:  raw.Content,
			Metadata: raw.Metadata,
		})
	}
	return docs, nil
}

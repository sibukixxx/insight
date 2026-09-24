// Package input is the provider-neutral input boundary of the engine
// (Issue #90). Inputs are inline documents, prepared Analytical Artifacts or
// references to raw artifacts that are read as bounded streams, so the
// engine never has to materialize every record of a project in memory.
//
// Input format is not research meaning: nothing here decides how evidence is
// interpreted (AnalysisMode) or how much to trust it.
package input

import (
	"context"
	"io"
	"strconv"
	"strings"

	"insight-lab/internal/domain"
)

// Kind names the variants a public caller can submit.
type Kind string

const (
	KindInlineDocument     Kind = "INLINE_DOCUMENT"
	KindAnalyticalArtifact Kind = "ANALYTICAL_ARTIFACT"
	KindRawArtifact        Kind = "RAW_ARTIFACT"
)

// Kinds lists every input kind this engine accepts.
func Kinds() []string {
	return []string{string(KindInlineDocument), string(KindAnalyticalArtifact), string(KindRawArtifact)}
}

// RawArtifactRef points at raw bytes held outside the engine. SHA256 and
// SizeBytes are claims until the engine has read the stream and verified
// them itself.
type RawArtifactRef struct {
	URI       string `json:"uri"`
	MediaType string `json:"mediaType"`
	Name      string `json:"name,omitempty"`
	Version   string `json:"version,omitempty"`
	SizeBytes int64  `json:"sizeBytes,omitempty"`
	SHA256    string `json:"sha256,omitempty"`
}

// Document metadata keys the engine writes for a registered raw artifact.
// They use the reserved public_ prefix, so a caller can never claim them.
const (
	MetadataKind        = "public_input_kind"
	MetadataRawURI      = "public_raw_uri"
	MetadataRawMedia    = "public_raw_media_type"
	MetadataRawName     = "public_raw_name"
	MetadataRawVersion  = "public_raw_version"
	MetadataRawSHA256   = "public_raw_sha256"
	MetadataRawSize     = "public_raw_size_bytes"
	MetadataPreparation = "public_raw_preparation"
)

// IsRawReference reports whether a stored document only references raw
// bytes. Its content is a descriptor and must never be analyzed as text.
func IsRawReference(d *domain.Document) bool {
	return d != nil && d.Metadata[MetadataKind] == string(KindRawArtifact)
}

// RefFromDocument restores the engine-verified reference of a raw artifact
// document.
func RefFromDocument(d *domain.Document) (RawArtifactRef, bool) {
	if !IsRawReference(d) {
		return RawArtifactRef{}, false
	}
	size, _ := strconv.ParseInt(d.Metadata[MetadataRawSize], 10, 64)
	return RawArtifactRef{
		URI: d.Metadata[MetadataRawURI], MediaType: d.Metadata[MetadataRawMedia], Name: d.Metadata[MetadataRawName],
		Version: d.Metadata[MetadataRawVersion], SizeBytes: size, SHA256: d.Metadata[MetadataRawSHA256],
	}, true
}

// Source yields documents one at a time. Next returns io.EOF when done.
// Implementations must not require the full input in memory.
type Source interface {
	Next(ctx context.Context) (*domain.Document, error)
}

// DocumentSource is the existing lightweight path: an in-memory slice of
// small documents served through the Source interface.
type DocumentSource struct {
	docs []*domain.Document
	next int
}

func NewDocumentSource(docs []*domain.Document) *DocumentSource { return &DocumentSource{docs: docs} }

func (s *DocumentSource) Next(ctx context.Context) (*domain.Document, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.next >= len(s.docs) {
		return nil, io.EOF
	}
	d := s.docs[s.next]
	s.next++
	return d, nil
}

// Shape is the bounded metadata the execution planner looks at.
type Shape struct {
	Documents        int   `json:"documents"`
	InlineBytes      int64 `json:"inlineBytes"`
	RawArtifacts     int   `json:"rawArtifacts"`
	RawBytes         int64 `json:"rawBytes"`
	RawToPrepare     int   `json:"rawToPrepare"`
	RawToPrepareByte int64 `json:"rawToPrepareBytes"`
}

// MeasureShape drains src and summarizes it without retaining documents.
func MeasureShape(ctx context.Context, src Source) (Shape, error) {
	var s Shape
	for {
		d, err := src.Next(ctx)
		if err == io.EOF {
			return s, nil
		}
		if err != nil {
			return Shape{}, err
		}
		if ref, ok := RefFromDocument(d); ok {
			s.RawArtifacts++
			s.RawBytes += ref.SizeBytes
			if strings.TrimSpace(d.Metadata[MetadataPreparation]) != "" {
				s.RawToPrepare++
				s.RawToPrepareByte += ref.SizeBytes
			}
			continue
		}
		s.Documents++
		s.InlineBytes += int64(len(d.Content))
	}
}

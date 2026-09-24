package input

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

var (
	// ErrUnavailable means no configured resolver can open the reference.
	ErrUnavailable = errors.New("input source unavailable")
	// ErrVerification means the bytes read do not match the reference.
	ErrVerification = errors.New("input verification failed")
)

// Resolver opens raw bytes for a reference. Implementations are adapters
// (local files, object stores, ...); Core ships only a local-file resolver.
type Resolver interface {
	Open(ctx context.Context, uri string) (io.ReadCloser, error)
}

// Resolvers dispatches by URI scheme. A nil or empty set resolves nothing.
type Resolvers map[string]Resolver

func (rs Resolvers) Open(ctx context.Context, uri string) (io.ReadCloser, error) {
	u, err := url.Parse(uri)
	if err != nil || u.Scheme == "" {
		return nil, fmt.Errorf("%w: %q is not an absolute URI", ErrUnavailable, uri)
	}
	r, ok := rs[strings.ToLower(u.Scheme)]
	if !ok {
		return nil, fmt.Errorf("%w: no resolver configured for scheme %q", ErrUnavailable, u.Scheme)
	}
	return r.Open(ctx, uri)
}

// FileResolver resolves "file:<relative/path>" inside Root. Absolute paths
// and paths escaping Root are refused, so a caller cannot read arbitrary
// files of the engine host.
type FileResolver struct {
	Root string
}

func (f FileResolver) Open(_ context.Context, uri string) (io.ReadCloser, error) {
	u, err := url.Parse(uri)
	if err != nil || !strings.EqualFold(u.Scheme, "file") || u.Host != "" {
		return nil, fmt.Errorf("%w: %q is not a file:<relative path> URI", ErrUnavailable, uri)
	}
	rel := u.Opaque
	if rel == "" {
		rel = u.Path
	}
	clean := filepath.Clean(filepath.FromSlash(rel))
	if f.Root == "" || rel == "" || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("%w: %q must be a relative path inside the configured root", ErrUnavailable, uri)
	}
	file, err := os.Open(filepath.Join(f.Root, clean))
	if err != nil {
		// The host path is not echoed: callers must not learn the layout.
		return nil, fmt.Errorf("%w: cannot open %q", ErrUnavailable, uri)
	}
	return file, nil
}

// VerifyingReader hashes and counts bytes as they stream and refuses to read
// past MaxBytes. It never buffers the stream.
type VerifyingReader struct {
	r        io.Reader
	h        hash.Hash
	n        int64
	maxBytes int64
}

func NewVerifyingReader(r io.Reader, maxBytes int64) *VerifyingReader {
	return &VerifyingReader{r: r, h: sha256.New(), maxBytes: maxBytes}
}

func (v *VerifyingReader) Read(p []byte) (int, error) {
	n, err := v.r.Read(p)
	v.n += int64(n)
	v.h.Write(p[:n])
	if v.maxBytes > 0 && v.n > v.maxBytes {
		return n, fmt.Errorf("%w: stream exceeds %d bytes", ErrVerification, v.maxBytes)
	}
	return n, err
}

func (v *VerifyingReader) SHA256() string   { return hex.EncodeToString(v.h.Sum(nil)) }
func (v *VerifyingReader) SizeBytes() int64 { return v.n }

// Check compares what was read with the reference's claims. Empty claims
// are filled from the engine's own measurement, never the other way round.
func (v *VerifyingReader) Check(ref RawArtifactRef) (RawArtifactRef, error) {
	sum := v.SHA256()
	if ref.SHA256 != "" && !strings.EqualFold(ref.SHA256, sum) {
		return ref, fmt.Errorf("%w: sha256 is %s, reference claims %s", ErrVerification, sum, ref.SHA256)
	}
	if ref.SizeBytes != 0 && ref.SizeBytes != v.n {
		return ref, fmt.Errorf("%w: size is %d bytes, reference claims %d", ErrVerification, v.n, ref.SizeBytes)
	}
	ref.SHA256, ref.SizeBytes = sum, v.n
	return ref, nil
}

// Verify streams the whole reference once and returns it with engine-measured
// sha256 and size.
func Verify(ctx context.Context, rs Resolver, ref RawArtifactRef, maxBytes int64) (RawArtifactRef, error) {
	rc, err := rs.Open(ctx, ref.URI)
	if err != nil {
		return ref, err
	}
	defer rc.Close()
	v := NewVerifyingReader(rc, maxBytes)
	if _, err := io.Copy(io.Discard, v); err != nil {
		return ref, err
	}
	return v.Check(ref)
}

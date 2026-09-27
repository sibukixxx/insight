package input

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
)

// CSV shards (#134). A HEAVY preparation splits one raw CSV into byte ranges
// that each hold whole records, found in a single bounded scan, so every
// partition reads only its own bytes instead of rescanning the whole file.
// A worker aggregates the header range followed by its shard range with the
// same AggregateCSV as a single pass, and the merged result is identical.

// MaxCSVHeaderBytes bounds the header record a shard plan keeps in memory.
const MaxCSVHeaderBytes = 1 << 20

// CSVShard is one byte range of whole records. ContentSHA256 is the sha256
// of the header bytes followed by the shard bytes: exactly what a worker
// aggregates, so it can prove it read the planned bytes.
type CSVShard struct {
	Offset        int64  `json:"offset"`
	Length        int64  `json:"length"`
	ContentSHA256 string `json:"contentSha256"`
}

// ID is stable for the same bytes and shard size, so a resumed job keeps
// the partitions that already succeeded.
func (s CSVShard) ID() string { return fmt.Sprintf("bytes-%d-%d", s.Offset, s.Length) }

// CSVShardPlan is the result of the planning scan.
type CSVShardPlan struct {
	HeaderLength int64
	SizeBytes    int64
	SHA256       string
	Shards       []CSVShard
}

// PlanCSVShards reads r once and cuts it after the first record terminator
// at or beyond every target bytes. A newline inside a quoted field is part
// of the record, never a boundary, which is sound because the CSV reader
// rejects bare quotes in unquoted fields. The header is the first record.
// A file without data rows has no shards.
func PlanCSVShards(ctx context.Context, r io.Reader, target int64) (CSVShardPlan, error) {
	if target <= 0 {
		return CSVShardPlan{}, errors.New("shard plan: target size must be positive")
	}
	file := sha256.New()
	var plan CSVShardPlan
	var header []byte
	inQuotes, inHeader := false, true
	var shard *CSVShard
	var shardHash hash.Hash
	var pos int64 // file offset of buf[0]

	buf := make([]byte, 64<<10)
	for {
		if err := ctx.Err(); err != nil {
			return CSVShardPlan{}, err
		}
		n, readErr := r.Read(buf)
		chunk := buf[:n]
		file.Write(chunk)
		start := 0 // first byte of chunk not yet written to header or shard
		for i, b := range chunk {
			if b == '"' {
				inQuotes = !inQuotes
			}
			if inHeader {
				if b == '\n' && !inQuotes {
					header = append(header, chunk[start:i+1]...)
					plan.HeaderLength, inHeader, start = pos+int64(i)+1, false, i+1
				}
				continue
			}
			if shard == nil {
				shard, shardHash = &CSVShard{Offset: pos + int64(i)}, sha256.New()
				shardHash.Write(header)
			}
			if end := pos + int64(i) + 1; b == '\n' && !inQuotes && end-shard.Offset >= target {
				shardHash.Write(chunk[start : i+1])
				shard.Length, shard.ContentSHA256 = end-shard.Offset, hex.EncodeToString(shardHash.Sum(nil))
				plan.Shards = append(plan.Shards, *shard)
				shard, start = nil, i+1
			}
		}
		if inHeader {
			header = append(header, chunk[start:]...)
		} else if shard != nil {
			shardHash.Write(chunk[start:])
		}
		if len(header) > MaxCSVHeaderBytes {
			return CSVShardPlan{}, fmt.Errorf("shard plan: the header record is larger than %d bytes", MaxCSVHeaderBytes)
		}
		pos += int64(n)
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return CSVShardPlan{}, readErr
		}
	}
	if inHeader {
		plan.HeaderLength = int64(len(header))
	}
	if shard != nil && pos > shard.Offset {
		shard.Length, shard.ContentSHA256 = pos-shard.Offset, hex.EncodeToString(shardHash.Sum(nil))
		plan.Shards = append(plan.Shards, *shard)
	}
	plan.SizeBytes, plan.SHA256 = pos, hex.EncodeToString(file.Sum(nil))
	return plan, nil
}

// RangeResolver opens a byte range of a reference without reading what
// precedes it. Resolvers that lack it are read from the start.
type RangeResolver interface {
	OpenRange(ctx context.Context, uri string, offset, length int64) (io.ReadCloser, error)
}

// OpenRange opens [offset, offset+length) of uri through rs, seeking when
// the resolver supports ranges and discarding the prefix otherwise.
func OpenRange(ctx context.Context, rs Resolver, uri string, offset, length int64) (io.ReadCloser, error) {
	if offset < 0 || length < 0 {
		return nil, fmt.Errorf("%w: negative byte range", ErrUnavailable)
	}
	if rr, ok := rs.(RangeResolver); ok {
		return rr.OpenRange(ctx, uri, offset, length)
	}
	rc, err := rs.Open(ctx, uri)
	if err != nil {
		return nil, err
	}
	if _, err := io.CopyN(io.Discard, rc, offset); err != nil {
		rc.Close()
		return nil, fmt.Errorf("%w: the input is shorter than the planned range", ErrVerification)
	}
	return readCloser{io.LimitReader(rc, length), rc}, nil
}

type readCloser struct {
	io.Reader
	io.Closer
}

func (rs Resolvers) OpenRange(ctx context.Context, uri string, offset, length int64) (io.ReadCloser, error) {
	r, err := rs.resolverFor(uri)
	if err != nil {
		return nil, err
	}
	return OpenRange(ctx, r, uri, offset, length)
}

func (f FileResolver) OpenRange(ctx context.Context, uri string, offset, length int64) (io.ReadCloser, error) {
	rc, err := f.Open(ctx, uri)
	if err != nil {
		return nil, err
	}
	file, ok := rc.(io.ReadSeekCloser)
	if !ok {
		rc.Close()
		return nil, fmt.Errorf("%w: %q is not seekable", ErrUnavailable, uri)
	}
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		file.Close()
		return nil, fmt.Errorf("%w: cannot seek %q", ErrUnavailable, uri)
	}
	return readCloser{io.LimitReader(file, length), file}, nil
}

// Package triage implements AI-assisted Data Triage (#92): a deterministic
// Dataset Profile, and an auditable, versioned Selection Plan that says which
// variables deterministic processing should look at for a research question.
//
// Triage never computes numbers that become evidence, never deletes source
// data and never decides research outcomes. Relevance proposed by a model is
// not causal evidence.
package triage

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	ProfilerVersion = "insight.dataset-profile/1"
	// DistinctCap bounds distinct-value counting per column.
	DistinctCap = 10000
	// SampleLimit bounds sample values kept per column.
	SampleLimit = 5
	// MaxSampleLength bounds each sample value.
	MaxSampleLength = 64
)

// ErrInvalid is returned for malformed input.
var ErrInvalid = errors.New("invalid triage input")

type ColumnType string

const (
	TypeInteger ColumnType = "INTEGER"
	TypeNumber  ColumnType = "NUMBER"
	TypeBoolean ColumnType = "BOOLEAN"
	TypeDate    ColumnType = "DATE"
	TypeString  ColumnType = "STRING"
	TypeEmpty   ColumnType = "EMPTY"
)

// ColumnProfile is bounded metadata about one column.
type ColumnProfile struct {
	Name           string     `json:"name"`
	Type           ColumnType `json:"type"`
	NonNullCount   int        `json:"nonNullCount"`
	NullCount      int        `json:"nullCount"`
	DistinctCount  int        `json:"distinctCount"`
	DistinctCapped bool       `json:"distinctCapped"`
	Min            string     `json:"min,omitempty"`
	Max            string     `json:"max,omitempty"`
	SampleValues   []string   `json:"sampleValues"`
}

// Profile is the deterministic part of a Dataset Profile: identical bytes
// always produce an identical Profile and Fingerprint.
type Profile struct {
	ContentSHA256   string          `json:"contentSha256"`
	RowCount        int             `json:"rowCount"`
	Columns         []ColumnProfile `json:"columns"`
	ProfilerVersion string          `json:"profilerVersion"`
}

// Fingerprint identifies the profile content.
func (p Profile) Fingerprint() string {
	b, _ := json.Marshal(p)
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}

var dateLayouts = []string{"2006-01-02", "2006-01", time.RFC3339, "2006/01/02"}

type columnAcc struct {
	col                  ColumnProfile
	distinct             map[string]struct{}
	isInt, isNum, isBool bool
	isDate               bool
	minNum, maxNum       float64
	minStr, maxStr       string
}

// ProfileCSV profiles CSV bytes. Column names must be unique and non-empty.
func ProfileCSV(data []byte) (Profile, error) {
	sum := sha256.Sum256(data)
	r := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))))
	r.FieldsPerRecord = -1
	header, err := r.Read()
	if err != nil {
		return Profile{}, fmt.Errorf("%w: csv header: %v", ErrInvalid, err)
	}
	accs := make([]*columnAcc, len(header))
	seen := map[string]bool{}
	for i, h := range header {
		name := strings.TrimSpace(h)
		if name == "" || seen[name] {
			return Profile{}, fmt.Errorf("%w: column %d name %q is empty or duplicated", ErrInvalid, i+1, name)
		}
		seen[name] = true
		accs[i] = &columnAcc{col: ColumnProfile{Name: name, SampleValues: []string{}}, distinct: map[string]struct{}{}, isInt: true, isNum: true, isBool: true, isDate: true}
	}
	rows := 0
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return Profile{}, fmt.Errorf("%w: csv row %d: %v", ErrInvalid, rows+2, err)
		}
		rows++
		for i, a := range accs {
			v := ""
			if i < len(rec) {
				v = strings.TrimSpace(rec[i])
			}
			a.add(v)
		}
	}
	p := Profile{ContentSHA256: "sha256:" + hex.EncodeToString(sum[:]), RowCount: rows, ProfilerVersion: ProfilerVersion}
	for _, a := range accs {
		p.Columns = append(p.Columns, a.finish())
	}
	return p, nil
}

func isNull(v string) bool {
	return v == "" || strings.EqualFold(v, "NA") || strings.EqualFold(v, "null") || strings.EqualFold(v, "N/A")
}

func (a *columnAcc) add(v string) {
	if isNull(v) {
		a.col.NullCount++
		return
	}
	a.col.NonNullCount++
	if _, ok := a.distinct[v]; !ok {
		if len(a.distinct) < DistinctCap {
			a.distinct[v] = struct{}{}
			if len(a.col.SampleValues) < SampleLimit {
				a.col.SampleValues = append(a.col.SampleValues, truncate(v))
			}
		} else {
			a.col.DistinctCapped = true
		}
	}
	if a.isInt {
		if _, err := strconv.ParseInt(v, 10, 64); err != nil {
			a.isInt = false
		}
	}
	if a.isNum {
		if f, err := strconv.ParseFloat(v, 64); err != nil {
			a.isNum = false
		} else {
			if a.col.NonNullCount == 1 || f < a.minNum {
				a.minNum = f
			}
			if a.col.NonNullCount == 1 || f > a.maxNum {
				a.maxNum = f
			}
		}
	}
	if a.isBool && !strings.EqualFold(v, "true") && !strings.EqualFold(v, "false") {
		a.isBool = false
	}
	if a.isDate && !parsesAsDate(v) {
		a.isDate = false
	}
	if a.minStr == "" || v < a.minStr {
		a.minStr = v
	}
	if v > a.maxStr {
		a.maxStr = v
	}
}

func (a *columnAcc) finish() ColumnProfile {
	c := a.col
	c.DistinctCount = len(a.distinct)
	sort.Strings(c.SampleValues)
	num := func(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }
	switch {
	case c.NonNullCount == 0:
		c.Type = TypeEmpty
	case a.isDate && !a.isNum:
		c.Type, c.Min, c.Max = TypeDate, truncate(a.minStr), truncate(a.maxStr)
	case a.isInt:
		c.Type, c.Min, c.Max = TypeInteger, num(a.minNum), num(a.maxNum)
	case a.isNum:
		c.Type, c.Min, c.Max = TypeNumber, num(a.minNum), num(a.maxNum)
	case a.isBool:
		c.Type = TypeBoolean
	default:
		c.Type, c.Min, c.Max = TypeString, truncate(a.minStr), truncate(a.maxStr)
	}
	return c
}

func parsesAsDate(v string) bool {
	for _, l := range dateLayouts {
		if _, err := time.Parse(l, v); err == nil {
			return true
		}
	}
	return false
}

func truncate(v string) string {
	r := []rune(v)
	if len(r) > MaxSampleLength {
		return string(r[:MaxSampleLength])
	}
	return v
}

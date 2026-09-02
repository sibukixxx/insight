// Spreadsheet intake: any CSV/TSV export (Zendesk tickets, a survey's
// free-text column, a review scrape) is mapped onto the document contract
// through a ColumnMapping the user confirms in a preview. Header names are
// only used to *suggest* the mapping; the mapping the user commits is what
// runs, and the project remembers it (IntakeProfile.ColumnMapping) so the
// next export from the same system imports without re-mapping.
package service

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"insight-lab/internal/domain"
	"insight-lab/internal/repository"
)

// Table is a parsed delimited file: a header row and data rows, all cells
// kept as strings.
type Table struct {
	Headers   []string
	Rows      [][]string
	Delimiter rune
}

// maxPreviewRows bounds the sample returned to the import preview.
const maxPreviewRows = 5

type TablePreview struct {
	Headers   []string             `json:"headers"`
	Sample    [][]string           `json:"sample"`
	RowCount  int                  `json:"rowCount"`
	Delimiter string               `json:"delimiter"`
	Suggested domain.ColumnMapping `json:"suggested"`
	// GuessedContent is true when no header looked like a content column
	// and the longest-text column was picked instead.
	GuessedContent bool `json:"guessedContent"`
}

// ParseTable reads a CSV or TSV (auto-detected from the header line),
// stripping a BOM. Rows shorter than the header are padded so a mapping
// never indexes out of range; longer rows keep their extra cells.
func ParseTable(r io.Reader) (*Table, error) {
	data, err := io.ReadAll(stripBOM(r))
	if err != nil {
		return nil, fmt.Errorf("読み込みに失敗しました: %w", err)
	}
	text := string(data)
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("ファイルが空です")
	}
	firstLine := text
	if i := strings.IndexByte(text, '\n'); i >= 0 {
		firstLine = text[:i]
	}
	delimiter := ','
	if strings.Count(firstLine, "\t") > strings.Count(firstLine, ",") {
		delimiter = '\t'
	}

	reader := csv.NewReader(strings.NewReader(text))
	reader.Comma = delimiter
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true

	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("ヘッダー行の読み込みに失敗しました: %w", err)
	}
	for i := range header {
		header[i] = strings.TrimSpace(header[i])
	}
	t := &Table{Headers: header, Delimiter: delimiter}
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%d行目の読み込みに失敗しました: %w", len(t.Rows)+2, err)
		}
		for len(record) < len(header) {
			record = append(record, "")
		}
		t.Rows = append(t.Rows, record)
	}
	return t, nil
}

func (t *Table) column(name string) int {
	want := strings.ToLower(strings.TrimSpace(name))
	if want == "" {
		return -1
	}
	for i, h := range t.Headers {
		if strings.ToLower(strings.TrimSpace(h)) == want {
			return i
		}
	}
	return -1
}

// Header vocabularies for mapping suggestions. Matching is exact on the
// lower-cased, trimmed header; a header that *contains* one of these is a
// weaker match used only when nothing matched exactly.
var (
	contentHeaders = []string{"content", "text", "body", "comment", "comments", "message", "answer", "feedback", "review", "description", "free_text", "freetext",
		"本文", "内容", "自由記述", "自由回答", "回答", "コメント", "発言", "意見", "感想", "要望", "記述", "レビュー", "本文テキスト", "問い合わせ内容", "お問い合わせ内容", "メッセージ", "テキスト"}
	titleHeaders    = []string{"title", "subject", "summary", "件名", "タイトル", "見出し", "題名", "要約"}
	idHeaders       = []string{"id", "no", "no.", "番号", "ticket_id", "ticket", "チケットid", "review_id", "row_id", "csv_id"}
	sourceHeaders   = []string{"source", "種別", "種類", "channel", "チャネル", "ソース", "データ種別"}
	metadataHeaders = map[string][]string{
		domain.MetaParticipantID: {"participant", "participant_id", "respondent", "respondent_id", "user_id", "customer_id", "回答者", "回答者id", "回答id", "回答番号", "顧客id", "ユーザーid", "参加者", "参加者id"},
		domain.MetaRole:          {"role", "job", "job_title", "position", "役職", "職種", "立場", "担当"},
		domain.MetaCompanySize:   {"company_size", "employees", "headcount", "従業員数", "社員数", "規模", "会社規模", "企業規模"},
		domain.MetaSegment:       {"segment", "industry", "category", "セグメント", "業種", "業界", "属性", "カテゴリ"},
		domain.MetaPlan:          {"plan", "tier", "プラン", "契約プラン"},
		domain.MetaDate:          {"date", "created_at", "created", "timestamp", "日付", "作成日", "回答日", "投稿日", "日時"},
		domain.MetaRating:        {"rating", "score", "stars", "star", "評価", "点数", "星", "満足度", "スコア"},
		domain.MetaVolume:        {"volume", "usage", "利用量", "件数", "月間件数", "利用頻度"},
	}
)

func matchHeader(h string, vocab []string) bool {
	h = strings.ToLower(strings.TrimSpace(h))
	for _, v := range vocab {
		if h == v {
			return true
		}
	}
	return false
}

func containsHeader(h string, vocab []string) bool {
	h = strings.ToLower(strings.TrimSpace(h))
	for _, v := range vocab {
		if len([]rune(v)) >= 2 && strings.Contains(h, v) {
			return true
		}
	}
	return false
}

// SuggestMapping guesses a ColumnMapping from header names. The legacy
// fixed layout (id,source,title,content) maps exactly as before. When no
// header names a content column, the column whose cells are longest on
// average is proposed and GuessedContent is set so the preview can say
// so.
func SuggestMapping(t *Table) (domain.ColumnMapping, bool) {
	m := domain.ColumnMapping{MetadataColumns: map[string]string{}}
	used := map[int]bool{}
	pick := func(vocab []string, weak bool) string {
		for i, h := range t.Headers {
			if used[i] {
				continue
			}
			if matchHeader(h, vocab) || (weak && containsHeader(h, vocab)) {
				used[i] = true
				return h
			}
		}
		return ""
	}
	m.ContentColumn = pick(contentHeaders, false)
	m.TitleColumn = pick(titleHeaders, false)
	m.IDColumn = pick(idHeaders, false)
	m.SourceColumn = pick(sourceHeaders, false)
	keys := make([]string, 0, len(metadataHeaders))
	for k := range metadataHeaders {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if col := pick(metadataHeaders[key], false); col != "" {
			m.MetadataColumns[col] = key
		}
	}
	if m.ContentColumn == "" {
		m.ContentColumn = pick(contentHeaders, true)
	}
	for _, key := range keys {
		if col := pick(metadataHeaders[key], true); col != "" {
			m.MetadataColumns[col] = key
		}
	}

	guessed := false
	if m.ContentColumn == "" && len(t.Rows) > 0 {
		best, bestLen := -1, 0
		for i := range t.Headers {
			if used[i] {
				continue
			}
			total := 0
			for _, row := range t.Rows {
				if i < len(row) {
					total += len([]rune(row[i]))
				}
			}
			if total > bestLen {
				best, bestLen = i, total
			}
		}
		if best >= 0 {
			m.ContentColumn = t.Headers[best]
			guessed = true
		}
	}
	if m.SourceColumn == "" {
		m.DefaultSource = domain.SourceInterview
	}
	return m, guessed
}

// PreviewTable parses r and returns what the mapping UI needs.
func PreviewTable(r io.Reader) (*TablePreview, error) {
	t, err := ParseTable(r)
	if err != nil {
		return nil, err
	}
	suggested, guessed := SuggestMapping(t)
	sample := t.Rows
	if len(sample) > maxPreviewRows {
		sample = sample[:maxPreviewRows]
	}
	if sample == nil {
		sample = [][]string{}
	}
	return &TablePreview{
		Headers: t.Headers, Sample: sample, RowCount: len(t.Rows),
		Delimiter: string(t.Delimiter), Suggested: suggested, GuessedContent: guessed,
	}, nil
}

// ImportOptions carries per-import context that is not part of the
// mapping itself.
type ImportOptions struct {
	// SpeakerRoles from the project's intake profile: a cell that itself
	// looks like a transcript (a ticket thread pasted into one field) gets
	// speaker spans, exactly like a paste would.
	SpeakerRoles map[string]domain.SpeakerRole
	// Masker, when set, is applied to each document's content (see pii.go).
	Masker func(string) (masked string, raw string, count int)
}

// ImportTable turns every row into a Document according to mapping.
// Rows with no content or an unknown source are skipped and reported,
// never silently dropped. The CSV's own id column is kept in
// metadata["csv_id"] for traceability; document IDs are always generated.
func ImportTable(ctx context.Context, documents repository.DocumentRepository, projectID string, t *Table, mapping domain.ColumnMapping, opts ImportOptions) (*ImportResult, error) {
	contentIdx := t.column(mapping.ContentColumn)
	if contentIdx < 0 {
		return nil, fmt.Errorf("本文の列 %q が見つかりません", mapping.ContentColumn)
	}
	titleIdx := t.column(mapping.TitleColumn)
	idIdx := t.column(mapping.IDColumn)
	sourceIdx := t.column(mapping.SourceColumn)
	if sourceIdx < 0 && !mapping.DefaultSource.Valid() {
		return nil, fmt.Errorf("種別の列がない場合は既定の種別（defaultSource）を指定してください")
	}
	if mapping.Provenance != "" && !mapping.Provenance.Valid() {
		return nil, fmt.Errorf("不正な出所: %q", mapping.Provenance)
	}
	type metaCol struct {
		idx int
		key string
	}
	var metaCols []metaCol
	for col, key := range mapping.MetadataColumns {
		key = strings.TrimSpace(key)
		if idx := t.column(col); idx >= 0 && key != "" {
			metaCols = append(metaCols, metaCol{idx: idx, key: key})
		}
	}
	sort.Slice(metaCols, func(i, j int) bool { return metaCols[i].idx < metaCols[j].idx })

	result := &ImportResult{}
	var toInsert []*domain.Document
	now := time.Now().UTC()
	for i, row := range t.Rows {
		rowNum := i + 1
		cell := func(idx int) string {
			if idx < 0 || idx >= len(row) {
				return ""
			}
			return strings.TrimSpace(row[idx])
		}
		content := strings.ReplaceAll(cell(contentIdx), "\r\n", "\n")
		if content == "" {
			result.Skipped++
			result.Errors = append(result.Errors, ImportRowError{Row: rowNum, Reason: "本文が空です"})
			continue
		}
		source := mapping.DefaultSource
		if sourceIdx >= 0 {
			if s := domain.SourceType(cell(sourceIdx)); s.Valid() {
				source = s
			} else if cell(sourceIdx) != "" || !mapping.DefaultSource.Valid() {
				result.Skipped++
				result.Errors = append(result.Errors, ImportRowError{Row: rowNum, Reason: fmt.Sprintf("不正なsource: %q", cell(sourceIdx))})
				continue
			}
		}
		provenance := mapping.Provenance
		if provenance == "" {
			provenance = domain.DefaultProvenance(source)
		}

		meta := map[string]string{}
		if v := cell(idIdx); v != "" {
			meta["csv_id"] = v
		}
		for _, mc := range metaCols {
			if v := cell(mc.idx); v != "" {
				meta[mc.key] = v
			}
		}

		raw := ""
		if opts.Masker != nil {
			var n int
			content, raw, n = opts.Masker(content)
			if n > 0 {
				result.Masked += n
			}
		}
		var spans []domain.Span
		if parsed := ParseTranscript(content, opts.SpeakerRoles); parsed.Detected {
			spans = parsed.Spans()
			result.WithSpeakers++
		}

		toInsert = append(toInsert, &domain.Document{
			ID: newID("doc"), ProjectID: projectID, Source: source, Provenance: provenance,
			Title: cell(titleIdx), Content: content, RawContent: raw, Spans: spans, Metadata: meta, CreatedAt: now,
		})
	}

	if len(toInsert) > 0 {
		if err := documents.CreateBatch(ctx, toInsert); err != nil {
			return nil, fmt.Errorf("保存に失敗しました: %w", err)
		}
	}
	result.Imported = len(toInsert)
	return result, nil
}

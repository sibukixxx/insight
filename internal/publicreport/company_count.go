package publicreport

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const (
	baselineYear = 2012
	latestYear   = 2021
)

type SourceMetadata struct {
	SourceName       string   `json:"source_name"`
	SourceURL        string   `json:"source_url"`
	Publisher        string   `json:"publisher"`
	RetrievedAt      string   `json:"retrieved_at"`
	CoveragePeriod   string   `json:"coverage_period"`
	GeographicScope  string   `json:"geographic_scope"`
	Unit             string   `json:"unit"`
	LicenseTerms     string   `json:"license_terms"`
	KnownLimitations []string `json:"known_limitations"`
}

type EnterpriseRecord struct {
	Geography      string `json:"geography"`
	Year           int    `json:"year"`
	EnterpriseCount int64  `json:"enterprise_count"`
}

type ChangeMetric struct {
	Geography    string  `json:"geography"`
	FromYear     int     `json:"from_year"`
	ToYear       int     `json:"to_year"`
	FromCount    int64   `json:"from_count"`
	ToCount      int64   `json:"to_count"`
	Delta        int64   `json:"delta"`
	PercentChange float64 `json:"percent_change"`
}

type Analysis struct {
	National          ChangeMetric `json:"national"`
	Tokyo             ChangeMetric `json:"tokyo"`
	TokyoShareFrom    float64      `json:"tokyo_share_from"`
	TokyoShareTo      float64      `json:"tokyo_share_to"`
	TokyoShareDeltaPP float64      `json:"tokyo_share_delta_pp"`
}

type EvidenceClaim struct {
	ClaimID         string   `json:"claim_id"`
	Claim           string   `json:"claim"`
	Source          string   `json:"source"`
	Dataset         string   `json:"dataset"`
	Calculation     string   `json:"calculation"`
	Confidence      string   `json:"confidence"`
	CounterEvidence []string `json:"counter_evidence"`
	Limitations     []string `json:"limitations"`
}

type EvidenceLedger struct {
	ResearchQuestion string          `json:"research_question"`
	GeneratedFrom    string          `json:"generated_from"`
	Claims           []EvidenceClaim `json:"claims"`
}

func LoadRecords(r io.Reader) ([]EnterpriseRecord, error) {
	reader := csv.NewReader(r)
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read CSV header: %w", err)
	}
	want := []string{"geography", "year", "enterprise_count"}
	if len(header) != len(want) {
		return nil, fmt.Errorf("CSV header must be %s", strings.Join(want, ","))
	}
	for i := range want {
		if strings.TrimSpace(strings.ToLower(header[i])) != want[i] {
			return nil, fmt.Errorf("CSV header must be %s", strings.Join(want, ","))
		}
	}

	var records []EnterpriseRecord
	seen := map[string]bool{}
	for row := 2; ; row++ {
		values, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read CSV row %d: %w", row, err)
		}
		if len(values) != 3 {
			return nil, fmt.Errorf("row %d: expected 3 columns", row)
		}
		geography := strings.TrimSpace(values[0])
		year, err := strconv.Atoi(strings.TrimSpace(values[1]))
		if err != nil {
			return nil, fmt.Errorf("row %d: invalid year: %w", row, err)
		}
		count, err := strconv.ParseInt(strings.TrimSpace(values[2]), 10, 64)
		if err != nil || count < 0 {
			return nil, fmt.Errorf("row %d: invalid enterprise_count", row)
		}
		if geography == "" {
			return nil, fmt.Errorf("row %d: geography is empty", row)
		}
		key := fmt.Sprintf("%s\x00%d", geography, year)
		if seen[key] {
			return nil, fmt.Errorf("row %d: duplicate geography/year %s/%d", row, geography, year)
		}
		seen[key] = true
		records = append(records, EnterpriseRecord{Geography: geography, Year: year, EnterpriseCount: count})
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("CSV has no records")
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].Geography == records[j].Geography {
			return records[i].Year < records[j].Year
		}
		return records[i].Geography < records[j].Geography
	})
	return records, nil
}

func Analyze(records []EnterpriseRecord) (Analysis, error) {
	index := make(map[string]int64, len(records))
	for _, record := range records {
		index[fmt.Sprintf("%s\x00%d", record.Geography, record.Year)] = record.EnterpriseCount
	}
	change := func(geography string) (ChangeMetric, error) {
		from, ok := index[fmt.Sprintf("%s\x00%d", geography, baselineYear)]
		if !ok {
			return ChangeMetric{}, fmt.Errorf("missing %s %d", geography, baselineYear)
		}
		to, ok := index[fmt.Sprintf("%s\x00%d", geography, latestYear)]
		if !ok {
			return ChangeMetric{}, fmt.Errorf("missing %s %d", geography, latestYear)
		}
		if from == 0 {
			return ChangeMetric{}, fmt.Errorf("%s %d count is zero", geography, baselineYear)
		}
		delta := to - from
		return ChangeMetric{
			Geography: geography, FromYear: baselineYear, ToYear: latestYear,
			FromCount: from, ToCount: to, Delta: delta,
			PercentChange: float64(delta) / float64(from) * 100,
		}, nil
	}

	national, err := change("全国")
	if err != nil {
		return Analysis{}, err
	}
	tokyo, err := change("東京都")
	if err != nil {
		return Analysis{}, err
	}
	if national.FromCount == 0 || national.ToCount == 0 {
		return Analysis{}, fmt.Errorf("national count cannot be zero")
	}
	shareFrom := float64(tokyo.FromCount) / float64(national.FromCount) * 100
	shareTo := float64(tokyo.ToCount) / float64(national.ToCount) * 100
	return Analysis{
		National: national, Tokyo: tokyo,
		TokyoShareFrom: shareFrom, TokyoShareTo: shareTo,
		TokyoShareDeltaPP: shareTo - shareFrom,
	}, nil
}

func Generate(inputPath, metadataPath, outputDir string) error {
	input, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("open source extract: %w", err)
	}
	defer input.Close()

	records, err := LoadRecords(input)
	if err != nil {
		return err
	}
	metadataBytes, err := os.ReadFile(metadataPath)
	if err != nil {
		return fmt.Errorf("read source metadata: %w", err)
	}
	var metadata SourceMetadata
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		return fmt.Errorf("parse source metadata: %w", err)
	}
	if metadata.SourceName == "" || metadata.SourceURL == "" || metadata.Publisher == "" || metadata.RetrievedAt == "" {
		return fmt.Errorf("source metadata is missing required provenance fields")
	}
	analysis, err := Analyze(records)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	outputs := map[string][]byte{}
	if outputs["normalized.csv"], err = renderNormalized(records); err != nil {
		return err
	}
	outputs["analysis.json"], err = json.MarshalIndent(analysis, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal analysis: %w", err)
	}
	outputs["analysis.json"] = append(outputs["analysis.json"], '\n')
	ledger := buildLedger(metadata, analysis)
	outputs["evidence-ledger.json"], err = json.MarshalIndent(ledger, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal evidence ledger: %w", err)
	}
	outputs["evidence-ledger.json"] = append(outputs["evidence-ledger.json"], '\n')
	outputs["report.md"] = []byte(renderReport(metadata, analysis))
	outputs["note-draft-source.md"] = []byte(renderNoteSource(metadata, analysis))
	outputs["sns-summary.md"] = []byte(renderSNSSummary(analysis))
	if outputs["insight-import.csv"], err = renderInsightImport(metadata, records); err != nil {
		return err
	}

	for name, content := range outputs {
		if err := os.WriteFile(filepath.Join(outputDir, name), content, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
	}
	return nil
}

func renderNormalized(records []EnterpriseRecord) ([]byte, error) {
	var b strings.Builder
	writer := csv.NewWriter(&b)
	if err := writer.Write([]string{"geography", "year", "enterprise_count"}); err != nil {
		return nil, err
	}
	for _, record := range records {
		if err := writer.Write([]string{record.Geography, strconv.Itoa(record.Year), strconv.FormatInt(record.EnterpriseCount, 10)}); err != nil {
			return nil, err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}
	return []byte(b.String()), nil
}

func renderInsightImport(metadata SourceMetadata, records []EnterpriseRecord) ([]byte, error) {
	var b strings.Builder
	writer := csv.NewWriter(&b)
	if err := writer.Write([]string{"id", "source", "title", "content"}); err != nil {
		return nil, err
	}
	for _, record := range records {
		id := fmt.Sprintf("enterprise-count-%s-%d", record.Geography, record.Year)
		title := fmt.Sprintf("%s %d年 企業数", record.Geography, record.Year)
		content := fmt.Sprintf(
			"Dataset observation: geography=%s; year=%d; enterprise_count=%d; unit=%s; source=%s; publisher=%s; retrieved_at=%s. This is a deterministic transcription of the published table and does not by itself establish a causal explanation.",
			record.Geography, record.Year, record.EnterpriseCount, metadata.Unit, metadata.SourceName, metadata.Publisher, metadata.RetrievedAt,
		)
		if err := writer.Write([]string{id, "dataset", title, content}); err != nil {
			return nil, err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}
	return []byte(b.String()), nil
}

func buildLedger(metadata SourceMetadata, analysis Analysis) EvidenceLedger {
	commonLimitations := append([]string(nil), metadata.KnownLimitations...)
	commonLimitations = append(commonLimitations,
		"2012年と2021年の離散時点比較であり、その間の年次経路を直接示さない。",
		"記述統計であり、企業数変化や地域差の因果要因を識別していない。",
	)
	return EvidenceLedger{
		ResearchQuestion: "日本の企業数は2012年から2021年に減ったのか。減少の一方で東京への集中は強まったのか？",
		GeneratedFrom:    "source_extract.csv + source_metadata.json; all arithmetic is deterministic",
		Claims: []EvidenceClaim{
			{
				ClaimID: "national-enterprise-count-2012-2021",
				Claim: fmt.Sprintf("全国の企業数は%d年の%sから%d年の%sへ%s（%.1f%%）変化した。",
					analysis.National.FromYear, formatInt(analysis.National.FromCount), analysis.National.ToYear,
					formatInt(analysis.National.ToCount), signedInt(analysis.National.Delta), analysis.National.PercentChange),
				Source: metadata.SourceURL, Dataset: metadata.SourceName,
				Calculation: fmt.Sprintf("%d - %d = %d; %d / %d * 100 = %.4f%%",
					analysis.National.ToCount, analysis.National.FromCount, analysis.National.Delta,
					analysis.National.Delta, analysis.National.FromCount, analysis.National.PercentChange),
				Confidence: "high_arithmetic_source_transcription",
				CounterEvidence: []string{"この集計だけでは、減少が法人の減少だけを意味するとは言えない。対象には個人事業者を含む。"},
				Limitations: commonLimitations,
			},
			{
				ClaimID: "tokyo-enterprise-count-2012-2021",
				Claim: fmt.Sprintf("東京都の企業数も%sから%sへ減少し、変化率は%.1f%%だった。",
					formatInt(analysis.Tokyo.FromCount), formatInt(analysis.Tokyo.ToCount), analysis.Tokyo.PercentChange),
				Source: metadata.SourceURL, Dataset: metadata.SourceName,
				Calculation: fmt.Sprintf("%d - %d = %d; %d / %d * 100 = %.4f%%",
					analysis.Tokyo.ToCount, analysis.Tokyo.FromCount, analysis.Tokyo.Delta,
					analysis.Tokyo.Delta, analysis.Tokyo.FromCount, analysis.Tokyo.PercentChange),
				Confidence: "high_arithmetic_source_transcription",
				CounterEvidence: []string{"東京都でも絶対数は増えていないため、「東京で企業が増えた」という説明はこのデータに反する。"},
				Limitations: commonLimitations,
			},
			{
				ClaimID: "tokyo-national-share-2012-2021",
				Claim: fmt.Sprintf("東京都の全国企業数に占める比率は%.2f%%から%.2f%%へ上昇し、差は%+.2fポイントだった。",
					analysis.TokyoShareFrom, analysis.TokyoShareTo, analysis.TokyoShareDeltaPP),
				Source: metadata.SourceURL, Dataset: metadata.SourceName,
				Calculation: fmt.Sprintf("%d/%d*100=%.6f%%; %d/%d*100=%.6f%%; difference=%+.6fpp",
					analysis.Tokyo.FromCount, analysis.National.FromCount, analysis.TokyoShareFrom,
					analysis.Tokyo.ToCount, analysis.National.ToCount, analysis.TokyoShareTo, analysis.TokyoShareDeltaPP),
				Confidence: "high_arithmetic_source_transcription",
				CounterEvidence: []string{
					"東京都の企業数自体は同期間に減少している。",
					"全国シェア上昇だけでは、本社移転・創業・廃業・人口移動などの因果メカニズムを特定できない。",
				},
				Limitations: commonLimitations,
			},
		},
	}
}

func renderReport(metadata SourceMetadata, a Analysis) string {
	return fmt.Sprintf(`# Research Question

日本の企業数は2012年から2021年に減ったのか。減少の一方で東京への集中は強まったのか？

## 結論

中小企業庁の同じ付属統計表に掲載された企業数では、全国の企業数は2012年の%sから2021年の%sへ%s、%.1f%%減少した。
東京都も%sから%sへ%.1f%%減っており、東京だけ企業数が増えたわけではない。
一方、東京都の全国シェアは%.2f%%から%.2f%%へ%+.2fポイント上昇した。
この期間については、「全国で企業数が減る」と「東京の相対的な比重が高まる」は同時に成立している。

## 使用データ

- データ: %s
- 公表者: %s
- 対象期間: %s
- 地理範囲: %s
- 単位: %s
- 取得日: %s
- 出典URL: %s

TechVitは公開表の値をsource extractへ転記し、差分・変化率・全国シェアを決定論的に再計算した。LLMは数値計算のsource of truthではない。

## まず何が起きているか

| 指標 | 2012 | 2021 | 変化 |
|---|---:|---:|---:|
| 全国企業数 | %s | %s | %s (%.1f%%) |
| 東京都企業数 | %s | %s | %s (%.1f%%) |
| 東京都の全国シェア | %.2f%% | %.2f%% | %+.2fポイント |

直接観測できるのは、全国と東京都の企業数がともに減っていること、そして東京都の減少率が全国より小さかったため全国シェアが上昇したことまでである。

## 予想と違ったこと

「東京への企業集中が進む」と聞くと、東京の企業数そのものが増えている状態を想像しやすい。しかしこのデータでは東京都の絶対数も減っている。Mismatchは、**絶対数の減少と相対シェアの上昇が同時に起きている**点にある。

## 考えられる説明

1. 全国的な企業数減少の中で、東京都は他地域より減少が緩やかだった。
2. 産業構成、人口・就業者構成、事業規模構成などの地域差が減少率の差に関係している。
3. 本社移転、開廃業、組織再編など複数のフローが同時に作用し、ストックの地域差として表れている。
4. 調査年・集計定義・対象範囲など統計上の条件差が一部に影響している可能性がある。

2〜4はこのP0データだけでは検証していない競合仮説であり、結論ではない。

## それを支持する証拠

- 全国: %s → %s、%s（%.1f%%）。
- 東京都: %s → %s、%s（%.1f%%）。
- 東京都の全国シェア: %.2f%% → %.2f%%、%+.2fポイント。
- 全国の減少率より東京都の減少率が小さいため、東京都の比率上昇は算術的に再現できる。

## 反対の証拠

- 東京都の企業数自体は%s減っている。「東京だけ企業数が増加した」という説明とは整合しない。
- シェア上昇だけでは、企業が東京へ移転したことや、東京で創業が特に増えたことは証明できない。
- この集計はストック比較であり、開業・廃業・移転というフローを直接観測していない。

## この分析では分からないこと

%s

加えて、2012年と2021年の比較だけでは途中の変動経路は分からない。相関・構成比の変化から政策や人口移動などの因果効果を断定することもできない。

## 現時点で言えること

このデータに基づく最も限定的なInsightは、**「企業数の全国的減少」と「東京の相対的比重の上昇」を分けて見る必要がある**ことだ。東京集中を議論するとき、絶対数と構成比を混同すると実態を誤って説明する。2021年時点までの比較では、東京は「増えた」のではなく「全国より減り方が小さかった」。

## 次に検証すること

- 47都道府県すべてで2012→2021の変化率を算出し、上位集中度や分布変化を確認する。
- 法人数だけに対象を絞るため、国税庁法人番号公表サイトの全件データによる別定義の現在ストックを検証する。
- 開業・廃業・本社移転を区別できるデータを追加し、ストック変化の内訳を検証する。
- 人口、就業者、産業構成を追加し、単純な地域シェア以外の説明を比較する。
- 調査方法・定義変更を原典で精査し、時点比較の同等性を確認する。

## Methodology

1. 公開統計表から対象行を source_extract.csv に保存する。
2. go run ./cmd/public-evidence-report が入力を検証し、normalized.csv を生成する。
3. 差分、変化率、東京都の全国シェアをGoコードで計算する。
4. 各公開主張を evidence-ledger.json の計算式と元データへ紐付ける。
5. 同じ計算結果からFull Report、note素材、SNS素材、Insight Lab再投入用CSVを生成する。
6. LLMを数値計算・Evidence生成のsource of truthには使わない。

## Sources

- %s — %s
- Retrieved: %s
- License / terms note: %s

---
Generated deterministically by Insight Lab Public Evidence Report Factory P0.
`,
		formatInt(a.National.FromCount), formatInt(a.National.ToCount), signedInt(a.National.Delta), -a.National.PercentChange,
		formatInt(a.Tokyo.FromCount), formatInt(a.Tokyo.ToCount), -a.Tokyo.PercentChange,
		a.TokyoShareFrom, a.TokyoShareTo, a.TokyoShareDeltaPP,
		metadata.SourceName, metadata.Publisher, metadata.CoveragePeriod, metadata.GeographicScope, metadata.Unit, metadata.RetrievedAt, metadata.SourceURL,
		formatInt(a.National.FromCount), formatInt(a.National.ToCount), signedInt(a.National.Delta), a.National.PercentChange,
		formatInt(a.Tokyo.FromCount), formatInt(a.Tokyo.ToCount), signedInt(a.Tokyo.Delta), a.Tokyo.PercentChange,
		a.TokyoShareFrom, a.TokyoShareTo, a.TokyoShareDeltaPP,
		formatInt(a.National.FromCount), formatInt(a.National.ToCount), signedInt(a.National.Delta), a.National.PercentChange,
		formatInt(a.Tokyo.FromCount), formatInt(a.Tokyo.ToCount), signedInt(a.Tokyo.Delta), a.Tokyo.PercentChange,
		a.TokyoShareFrom, a.TokyoShareTo, a.TokyoShareDeltaPP,
		signedInt(a.Tokyo.Delta),
		renderLimitations(metadata.KnownLimitations),
		metadata.SourceName, metadata.SourceURL, metadata.RetrievedAt, metadata.LicenseTerms,
	)
}

func renderNoteSource(metadata SourceMetadata, a Analysis) string {
	return fmt.Sprintf(`# note draft source

## 社会的な問い

「日本では会社が減っている」と「東京一極集中」は矛盾しないのか。

## 使える事実

- 全国企業数: %s → %s（%.1f%%）
- 東京都企業数: %s → %s（%.1f%%）
- 東京都の全国シェア: %.2f%% → %.2f%%（%+.2fポイント）

## 記事の核

東京の企業数も減っている。それでも全国より減少率が小さいため、全国に占める東京の比率は上がる。
「東京集中」を絶対数の増加と同義にすると、この違いを見落とす。

## 書くときの注意

- 「法人だけ」の統計として書かない。原表の企業数定義に従う。
- 東京への移転が原因だと断定しない。
- 開廃業、産業構成、人口などは次の検証課題として扱う。
- TechVit独自調査ではなく、%sの公開データをTechVitが再集計した一次分析と明記する。

Source: %s
Retrieved: %s
`,
		formatInt(a.National.FromCount), formatInt(a.National.ToCount), a.National.PercentChange,
		formatInt(a.Tokyo.FromCount), formatInt(a.Tokyo.ToCount), a.Tokyo.PercentChange,
		a.TokyoShareFrom, a.TokyoShareTo, a.TokyoShareDeltaPP,
		metadata.Publisher, metadata.SourceURL, metadata.RetrievedAt,
	)
}

func renderSNSSummary(a Analysis) string {
	return fmt.Sprintf(`結論:
2012→2021で企業数は全国で%.1f%%減少。東京都も%.1f%%減った。

意外だった数字:
東京の絶対数は減っているのに、全国シェアは%.2f%%→%.2f%%へ%+.2fポイント上昇。

一言解釈:
「企業数の減少」と「東京の相対的比重の上昇」は同時に起こり得る。東京集中=東京の会社数が増える、ではない。

Full Report:
[公開URLを設定]
`,
		-a.National.PercentChange, -a.Tokyo.PercentChange, a.TokyoShareFrom, a.TokyoShareTo, a.TokyoShareDeltaPP,
	)
}

func renderLimitations(values []string) string {
	if len(values) == 0 {
		return "- 原典の定義・調査方法を公開前に再確認する必要がある。"
	}
	var b strings.Builder
	for _, value := range values {
		fmt.Fprintf(&b, "- %s\n", value)
	}
	return strings.TrimSpace(b.String())
}

func formatInt(value int64) string {
	negative := value < 0
	if negative {
		value = -value
	}
	s := strconv.FormatInt(value, 10)
	if len(s) > 3 {
		var out []byte
		first := len(s) % 3
		if first == 0 {
			first = 3
		}
		out = append(out, s[:first]...)
		for i := first; i < len(s); i += 3 {
			out = append(out, ',')
			out = append(out, s[i:i+3]...)
		}
		s = string(out)
	}
	if negative {
		return "-" + s
	}
	return s
}

func signedInt(value int64) string {
	if value > 0 {
		return "+" + formatInt(value)
	}
	return formatInt(value)
}

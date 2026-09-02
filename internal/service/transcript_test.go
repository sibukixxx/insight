package service

import (
	"strings"
	"testing"

	"insight-lab/internal/domain"
)

func turnByRole(p TranscriptParse, role domain.SpeakerRole) []Turn {
	var out []Turn
	for _, t := range p.Turns {
		if t.Role == role {
			out = append(out, t)
		}
	}
	return out
}

func TestParseTranscriptKnownJapaneseLabels(t *testing.T) {
	content := "面接官: 請求書の作成にどのくらいかかりますか？\n回答者: 半日くらいです。\nテンプレートはあるんですが、検算に時間がかかります。\n\n面接官: なぜ検算を？\n回答者: 間違えるのが怖いので。"
	p := ParseTranscript(content, nil)
	if !p.Detected {
		t.Fatal("transcript should be detected")
	}
	if len(p.Turns) != 4 {
		t.Fatalf("turns = %d, want 4: %+v", len(p.Turns), p.Turns)
	}
	customer := turnByRole(p, domain.RoleCustomer)
	if len(customer) != 2 || customer[0].Guessed {
		t.Fatalf("customer turns = %+v", customer)
	}
	// Continuation line belongs to the previous turn; the label is not
	// part of the span; offsets index the original content.
	runes := []rune(content)
	got := string(runes[customer[0].Start:customer[0].End])
	if got != "半日くらいです。\nテンプレートはあるんですが、検算に時間がかかります。" {
		t.Errorf("first customer span = %q", got)
	}
	if strings.Contains(got, "回答者") {
		t.Error("span must not include the speaker label")
	}
	spans := p.Spans()
	if len(spans) != 4 || spans[0].Role != domain.RoleInterviewer || spans[0].Speaker != "面接官" {
		t.Errorf("Spans() = %+v", spans)
	}
	if len(p.Warnings) != 0 {
		t.Errorf("no warnings expected for known labels, got %v", p.Warnings)
	}
}

func TestParseTranscriptQAStyleAndTimestamps(t *testing.T) {
	content := "Q. 導入のきっかけは？\nA. 前任者が辞めたので。\n[00:01:12] Q. 困ったことは？\n00:02:30 A. 設定が古いままで再送する羽目に。"
	p := ParseTranscript(content, nil)
	if !p.Detected || len(p.Turns) != 4 {
		t.Fatalf("detected=%v turns=%+v", p.Detected, p.Turns)
	}
	if p.Turns[2].Role != domain.RoleInterviewer || p.Turns[3].Role != domain.RoleCustomer {
		t.Errorf("roles = %q, %q", p.Turns[2].Role, p.Turns[3].Role)
	}
	if p.Turns[3].Text != "設定が古いままで再送する羽目に。" {
		t.Errorf("timestamped turn text = %q", p.Turns[3].Text)
	}
}

func TestParseTranscriptUnknownNamesUseTalkTimeHeuristic(t *testing.T) {
	content := "田中: 今日はよろしくお願いします。まず普段の業務を教えてください。\n佐藤: はい。毎月月末に請求書を作っています。金額を打ち込んだあと必ず電卓で検算します。ソフトの計算を疑っているわけではないんですが、間違えたときの影響が大きいので。\n田中: なるほど。\n佐藤: はい、そこだけは外せません。"
	p := ParseTranscript(content, nil)
	if !p.Detected {
		t.Fatal("two alternating unknown speakers should be detected as a transcript")
	}
	var sato, tanaka *SpeakerSummary
	for i := range p.Speakers {
		switch p.Speakers[i].Label {
		case "佐藤":
			sato = &p.Speakers[i]
		case "田中":
			tanaka = &p.Speakers[i]
		}
	}
	if sato == nil || tanaka == nil {
		t.Fatalf("speakers = %+v", p.Speakers)
	}
	if sato.Role != domain.RoleCustomer || tanaka.Role != domain.RoleInterviewer || !sato.Guessed {
		t.Errorf("heuristic roles: 佐藤=%q(guessed %v) 田中=%q", sato.Role, sato.Guessed, tanaka.Role)
	}
	if len(p.Warnings) == 0 {
		t.Error("guessed roles must produce a warning for the preview")
	}

	// The profile's mapping wins over the heuristic.
	p2 := ParseTranscript(content, map[string]domain.SpeakerRole{"田中": domain.RoleCustomer, "佐藤": domain.RoleInterviewer})
	for _, s := range p2.Speakers {
		if s.Guessed {
			t.Errorf("profile-assigned speaker %q should not be marked guessed", s.Label)
		}
		if s.Label == "田中" && s.Role != domain.RoleCustomer {
			t.Errorf("profile mapping ignored for 田中: %q", s.Role)
		}
	}
}

func TestParseTranscriptSupportThread(t *testing.T) {
	content := "顧客: 請求書のPDFが文字化けします。\nサポート: ご迷惑をおかけしております。ブラウザは何をお使いですか？\n顧客: Safariです。急いでいるので早く直してほしいです。"
	p := ParseTranscript(content, nil)
	if !p.Detected {
		t.Fatal("support thread should be detected")
	}
	if len(turnByRole(p, domain.RoleAgent)) != 1 || len(turnByRole(p, domain.RoleCustomer)) != 2 {
		t.Errorf("roles: %+v", p.Turns)
	}
}

func TestParseTranscriptRejectsProse(t *testing.T) {
	prose := "毎月月末になると請求書の作成に半日くらいかかります。\n注意: 金額の入力は慎重に。\nそれでも検算は欠かしません。"
	p := ParseTranscript(prose, nil)
	if p.Detected {
		t.Errorf("a single incidental 'label:' line must not make prose a transcript: %+v", p.Turns)
	}
	if len(ParseTranscript("", nil).Turns) != 0 {
		t.Error("empty content")
	}
}

func TestParseTranscriptWarnsWhenNoCustomer(t *testing.T) {
	content := "面接官: 質問です。\n司会: 次の方どうぞ。"
	p := ParseTranscript(content, nil)
	if !p.Detected {
		t.Fatal("known labels should detect")
	}
	found := false
	for _, w := range p.Warnings {
		if strings.Contains(w, "回答者") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a no-customer warning, got %v", p.Warnings)
	}
}

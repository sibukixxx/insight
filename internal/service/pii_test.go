package service

import (
	"strings"
	"testing"
)

func TestMaskBuiltinPatterns(t *testing.T) {
	m := NewMasker(nil)
	cases := []struct {
		name, in, want string
		kind           MaskKind
	}{
		{"email", "連絡先は taro.yamada@example.co.jp です", "連絡先は [メールアドレス] です", MaskEmail},
		{"url", "詳細は https://example.com/path?x=1 を見てください", "詳細は [URL] を見てください", MaskURL},
		{"phone hyphen", "電話は 03-1234-5678 まで", "電話は [電話番号] まで", MaskPhone},
		{"mobile", "携帯 090-1234-5678 に", "携帯 [電話番号] に", MaskPhone},
		{"phone plain", "TEL 0312345678", "TEL [電話番号]", MaskPhone},
		{"phone intl", "+81-3-1234-5678", "[電話番号]", MaskPhone},
		{"postal", "〒100-0001 千代田区", "[郵便番号] 千代田区", MaskPostal},
		{"card", "カード 4111-1111-1111-1111 で", "カード [カード番号] で", MaskCard},
		{"name san", "田中さんに確認してから送ります", "[氏名]さんに確認してから送ります", MaskName},
		{"name sama", "山田様宛の請求書", "[氏名]様宛の請求書", MaskName},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := m.Mask(tc.in)
			if r.Masked != tc.want {
				t.Errorf("Mask(%q) = %q, want %q", tc.in, r.Masked, tc.want)
			}
			if r.ByKind[tc.kind] != 1 || r.Count != 1 || r.Raw != tc.in {
				t.Errorf("result bookkeeping = %+v", r)
			}
		})
	}
}

func TestMaskLeavesOrdinaryTextAlone(t *testing.T) {
	m := NewMasker(nil)
	for _, s := range []string{
		"毎月月末になると請求書の作成に半日くらいかかります。",
		"2026-08-01 に 150件 発行し、合計 1,200,000円 でした。",
		"お客様との関係が一気に悪くなるので。皆様にご迷惑を。",
		"経理部長と営業担当に確認しました。",
		"彼氏に相談した。奥様は反対した。",
		"消費税の端数処理は 10% で、税率は 8% のものもある。",
	} {
		if r := m.Mask(s); r.Masked != s || r.Count != 0 || r.Raw != "" {
			t.Errorf("Mask(%q) changed text: %+v", s, r)
		}
	}
}

func TestMaskDictionaryLongestFirst(t *testing.T) {
	m := NewMasker([]string{"サンプル", "株式会社サンプル", "", "  ", "サンプル"})
	r := m.Mask("株式会社サンプルの田中さんから、サンプル画面について問い合わせ")
	if !strings.HasPrefix(r.Masked, "[固有名詞]の[氏名]さん") {
		t.Errorf("Masked = %q", r.Masked)
	}
	if strings.Contains(r.Masked, "株式会社") {
		t.Errorf("longer term should be masked as a whole: %q", r.Masked)
	}
	if r.ByKind[MaskTerm] != 2 || r.ByKind[MaskName] != 1 || r.Count != 3 {
		t.Errorf("counts = %+v", r.ByKind)
	}
}

func TestMaskIsDeterministicAndOffsetsUsable(t *testing.T) {
	m := NewMasker([]string{"山田"})
	in := "山田: 090-1234-5678 に電話しました。\n担当: 承知しました。"
	a, b := m.Mask(in), m.Mask(in)
	if a.Masked != b.Masked {
		t.Fatal("masking must be deterministic")
	}
	// Speaker parsing on the masked text still works; spans index the
	// masked content, which is what gets stored and shown.
	p := ParseTranscript(a.Masked, nil)
	if !p.Detected || len(p.Turns) != 2 {
		t.Fatalf("transcript on masked text: %+v", p)
	}
	runes := []rune(a.Masked)
	if got := string(runes[p.Turns[0].Start:p.Turns[0].End]); got != "[電話番号] に電話しました。" {
		t.Errorf("first turn = %q", got)
	}
	if p.Turns[0].Speaker != "[固有名詞]" {
		t.Errorf("dictionary term in a speaker label is masked too: %q", p.Turns[0].Speaker)
	}
}

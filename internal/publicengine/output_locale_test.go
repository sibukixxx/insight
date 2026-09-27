package publicengine

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// #125: callers may request ja-JP or en-US for new model-generated text.
// There is no default locale: omission keeps the source-language rule.
func TestEngineAdvertisesSupportedOutputLocalesWithoutDefault(t *testing.T) {
	info := newTestEngine(t).Engine()
	if got := strings.Join(info.SupportedOutputLocales, ","); got != "ja-JP,en-US" {
		t.Fatalf("supportedOutputLocales = %q", got)
	}
	body, _ := json.Marshal(info)
	if strings.Contains(string(body), "defaultOutputLocale") {
		t.Fatalf("an omitted locale is not a locale; no default may be advertised: %s", body)
	}
}

func TestStartAnalysisRejectsUnsupportedOutputLocale(t *testing.T) {
	e := newTestEngine(t)
	subjectID := createTestSubject(t, e)
	for i, locale := range []string{"ja", "ja-jp", "fr-FR"} {
		_, _, err := e.StartAnalysis(context.Background(), subjectID, StartAnalysisRequest{
			ContractVersion: "1", IdempotencyKey: "bad-locale-" + string(rune('a'+i)), OutputLocale: locale,
		})
		if got := AsError(err); got.Code != CodeInvalidRequest {
			t.Fatalf("%q: code = %s (%s), want %s", locale, got.Code, got.Message, CodeInvalidRequest)
		}
	}
}

func TestStartAnalysisEchoesRequestedOutputLocaleAndOmitsItWhenNotRequested(t *testing.T) {
	e := newTestEngine(t)
	subjectID := createTestSubject(t, e)
	for key, locale := range map[string]string{"locale-ja": "ja-JP", "locale-omitted": ""} {
		_, body, err := e.StartAnalysis(context.Background(), subjectID, StartAnalysisRequest{ContractVersion: "1", IdempotencyKey: key, OutputLocale: locale})
		if err != nil {
			t.Fatal(err)
		}
		var run AnalysisRun
		if err := json.Unmarshal(body, &run); err != nil {
			t.Fatal(err)
		}
		if run.OutputLocale != locale {
			t.Fatalf("%s: outputLocale = %q, want %q", key, run.OutputLocale, locale)
		}
		if locale == "" && strings.Contains(string(body), "outputLocale") {
			t.Fatalf("an omitted locale must not be serialized: %s", body)
		}
	}
}

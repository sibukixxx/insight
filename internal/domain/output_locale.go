package domain

// OutputLocale is the language a caller explicitly requested for new
// model-generated explanatory text (#125). It is generation context, not
// research meaning: it is independent from AnalysisMode, ReasoningProfile,
// ResearchStage, ExecutionMode, ExecutionProfile and any UI display locale.
//
// Empty means "not requested": generated text follows the predominant
// language of the source material or research question, as before. It is
// never resolved to a locale the engine cannot know. Source quotes, numbers,
// units, IDs and enum codes are never translated whatever the locale.
type OutputLocale string

const (
	OutputLocaleJaJP OutputLocale = "ja-JP"
	OutputLocaleEnUS OutputLocale = "en-US"
)

// SupportedOutputLocales lists the explicit values callers may request.
var SupportedOutputLocales = []OutputLocale{OutputLocaleJaJP, OutputLocaleEnUS}

// Valid accepts the unset value and the exact supported tags. Other spellings
// ("ja", "ja-jp") are rejected rather than guessed.
func (l OutputLocale) Valid() bool {
	if l == "" {
		return true
	}
	for _, s := range SupportedOutputLocales {
		if l == s {
			return true
		}
	}
	return false
}

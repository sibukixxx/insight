package domain

import "testing"

func TestOutputLocaleValidAcceptsOnlyExplicitTagsAndUnset(t *testing.T) {
	for _, tc := range []struct {
		locale OutputLocale
		want   bool
	}{
		{"", true}, {"ja-JP", true}, {"en-US", true},
		{"ja", false}, {"en", false}, {"ja-jp", false}, {"fr-FR", false}, {" ja-JP", false},
	} {
		if got := tc.locale.Valid(); got != tc.want {
			t.Errorf("OutputLocale(%q).Valid() = %v, want %v", tc.locale, got, tc.want)
		}
	}
}

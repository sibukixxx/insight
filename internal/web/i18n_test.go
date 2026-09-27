package web

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The browser UI shows every screen string through dictionaries under
// dist/locales (#124). These tests keep the dictionaries and app.js in step:
// a key used on screen must exist, every locale must translate every key
// with the same parameters, and no screen copy may stay hard-coded.

var locales = []string{"en", "ja"}

func loadDictionary(t *testing.T, locale string) map[string]string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("dist", "locales", locale+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var dict map[string]string
	if err := json.Unmarshal(raw, &dict); err != nil {
		t.Fatalf("%s.json: %v", locale, err)
	}
	return dict
}

func readAppJS(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("dist", "app.js"))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

var placeholderPattern = regexp.MustCompile(`\{(\w+)\}`)

func placeholders(s string) string {
	var names []string
	for _, m := range placeholderPattern.FindAllStringSubmatch(s, -1) {
		names = append(names, m[1])
	}
	sort.Strings(names)
	return strings.Join(names, ",")
}

func TestLocaleDictionariesHaveSameKeysAndPlaceholdersWhenCompared(t *testing.T) {
	en := loadDictionary(t, "en")
	for _, locale := range locales[1:] {
		other := loadDictionary(t, locale)
		for key, text := range en {
			translated, ok := other[key]
			if !ok {
				t.Errorf("%s.json is missing %q", locale, key)
				continue
			}
			if strings.TrimSpace(translated) == "" {
				t.Errorf("%s.json has an empty translation for %q", locale, key)
			}
			if placeholders(text) != placeholders(translated) {
				t.Errorf("%q placeholders differ: en {%s} vs %s {%s}", key, placeholders(text), locale, placeholders(translated))
			}
		}
		for key := range other {
			if _, ok := en[key]; !ok {
				t.Errorf("%s.json has %q, which en.json does not define", locale, key)
			}
		}
	}
}

var (
	textCallPattern = regexp.MustCompile(`\bth?\("([^"]+)"`)
	keyLiteral      = regexp.MustCompile(`"([a-z][a-zA-Z]*(?:\.[a-zA-Z]+)+)"`)
)

func TestAppJSUsesOnlyDefinedKeysAndEveryKeyIsUsedWhenScanned(t *testing.T) {
	en := loadDictionary(t, "en")
	js := readAppJS(t)
	namespaces := map[string]bool{}
	for key := range en {
		namespaces[strings.SplitN(key, ".", 2)[0]] = true
	}
	used := map[string]bool{}
	for _, m := range textCallPattern.FindAllStringSubmatch(js, -1) {
		used[m[1]] = true
	}
	for _, m := range keyLiteral.FindAllStringSubmatch(js, -1) {
		if namespaces[strings.SplitN(m[1], ".", 2)[0]] {
			used[m[1]] = true
		}
	}
	for key := range used {
		if _, ok := en[key]; !ok {
			t.Errorf("app.js uses %q, which en.json does not define", key)
		}
	}
	for key := range en {
		if !strings.Contains(js, `"`+key+`"`) {
			t.Errorf("en.json defines %q, which app.js never uses", key)
		}
	}
}

// Text between tags and user-facing attributes must come from dictionaries.
// Brand, sample values and code tokens are the only literals allowed.
func TestAppJSHasNoHardCodedScreenCopyWhenScanned(t *testing.T) {
	js := readAppJS(t)
	allowed := map[string]bool{
		"Insight Lab": true, "https://api.openai.com/v1": true, "gpt-5": true, "sk-...": true,
		"make build-demo": true,
	}
	word := regexp.MustCompile(`[A-Za-z]{2,}`)
	textNode := regexp.MustCompile(`>([^<>]*)<`)
	attribute := regexp.MustCompile(`\b(?:placeholder|title|aria-label|alt)="([^"]*)"`)
	check := func(kind, s string) {
		stripped := regexp.MustCompile(`\$\{[^}]*\}?`).ReplaceAllString(s, "")
		stripped = regexp.MustCompile(`&[a-z]+;`).ReplaceAllString(stripped, "")
		stripped = strings.TrimSpace(stripped)
		if stripped == "" || allowed[stripped] || !word.MatchString(stripped) {
			return
		}
		// Template fragments such as `${cond ? "x" : ""}` leave code, not copy.
		if strings.ContainsAny(stripped, "(){};=`") {
			return
		}
		t.Errorf("hard-coded %s copy in app.js: %q", kind, stripped)
	}
	for _, m := range textNode.FindAllStringSubmatch(js, -1) {
		check("text", m[1])
	}
	for _, m := range attribute.FindAllStringSubmatch(js, -1) {
		check("attribute", m[1])
	}
}

func TestHandlerServesLocaleDictionariesAsJSONWhenRequested(t *testing.T) {
	server := httptest.NewServer(Handler())
	defer server.Close()
	for _, locale := range locales {
		res, err := http.Get(server.URL + "/locales/" + locale + ".json")
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(res.Body)
		res.Body.Close()
		var dict map[string]string
		if res.StatusCode != http.StatusOK || json.Unmarshal(body, &dict) != nil || len(dict) == 0 {
			t.Fatalf("/locales/%s.json: status %d, body is not a dictionary: %.80s", locale, res.StatusCode, body)
		}
	}
}

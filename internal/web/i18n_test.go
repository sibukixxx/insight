package web

import (
	"encoding/json"
	"io"
	"io/fs"
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
// dist/locales (#124), built from web/public/locales. These tests keep the
// dictionaries and the TypeScript sources under web/src in step:
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

// frontendSource concatenates the TypeScript UI sources under web/src
// (tests excluded): the sources the dist/ bundle is built from.
func frontendSource(t *testing.T) string {
	t.Helper()
	var b strings.Builder
	root := filepath.Join("..", "..", "web", "src")
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		ext := filepath.Ext(path)
		if (ext != ".ts" && ext != ".tsx") || strings.Contains(path, ".test.") || strings.Contains(path, string(filepath.Separator)+"test"+string(filepath.Separator)) {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		b.WriteString("\n// file: " + filepath.ToSlash(path) + "\n")
		b.Write(raw)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if b.Len() == 0 {
		t.Fatal("no frontend sources found under web/src")
	}
	return b.String()
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
	textCallPattern = regexp.MustCompile(`\bt(?:Rich)?\("([^"]+)"`)
	keyLiteral      = regexp.MustCompile(`"([a-z][a-zA-Z]*(?:\.[a-zA-Z0-9]+)+)"`)
)

func TestFrontendUsesOnlyDefinedKeysAndEveryKeyIsUsedWhenScanned(t *testing.T) {
	en := loadDictionary(t, "en")
	src := frontendSource(t)
	namespaces := map[string]bool{}
	for key := range en {
		namespaces[strings.SplitN(key, ".", 2)[0]] = true
	}
	used := map[string]bool{}
	for _, m := range textCallPattern.FindAllStringSubmatch(src, -1) {
		used[m[1]] = true
	}
	for _, m := range keyLiteral.FindAllStringSubmatch(src, -1) {
		if namespaces[strings.SplitN(m[1], ".", 2)[0]] {
			used[m[1]] = true
		}
	}
	for key := range used {
		if _, ok := en[key]; !ok {
			t.Errorf("web/src uses %q, which en.json does not define", key)
		}
	}
	for key := range en {
		if !strings.Contains(src, `"`+key+`"`) {
			t.Errorf("en.json defines %q, which web/src never uses", key)
		}
	}
}

// JSX text is checked by ESLint (react/jsx-no-literals). User-facing
// attributes are checked here so `make test` guards them without Node:
// brand, sample values and code tokens are the only literals allowed.
func TestFrontendHasNoHardCodedAttributeCopyWhenScanned(t *testing.T) {
	src := frontendSource(t)
	allowed := map[string]bool{
		"https://api.openai.com/v1": true, "gpt-5": true, "sk-...": true,
	}
	word := regexp.MustCompile(`[A-Za-z]{2,}`)
	attribute := regexp.MustCompile(`\b(?:placeholder|title|aria-label|alt|label|summary)="([^"]*)"`)
	for _, m := range attribute.FindAllStringSubmatch(src, -1) {
		text := strings.TrimSpace(m[1])
		if text == "" || allowed[text] || !word.MatchString(text) {
			continue
		}
		t.Errorf("hard-coded attribute copy in web/src: %q", text)
	}
}

// dist/locales is copied from web/public/locales by the build; a stale copy
// would serve different text than the sources define.
func TestDistLocalesMatchFrontendSourcesWhenCompared(t *testing.T) {
	for _, locale := range locales {
		built, err := os.ReadFile(filepath.Join("dist", "locales", locale+".json"))
		if err != nil {
			t.Fatal(err)
		}
		source, err := os.ReadFile(filepath.Join("..", "..", "web", "public", "locales", locale+".json"))
		if err != nil {
			t.Fatal(err)
		}
		if string(built) != string(source) {
			t.Errorf("dist/locales/%s.json differs from web/public/locales/%s.json; run make web-build", locale, locale)
		}
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

var assetReference = regexp.MustCompile(`(?:src|href)="/(assets/[^"]+)"`)

// The embedded index.html must reference bundle files that are embedded too.
func TestHandlerServesIndexWhoseAssetsAreEmbeddedWhenRequested(t *testing.T) {
	server := httptest.NewServer(Handler())
	defer server.Close()
	res, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	refs := assetReference.FindAllStringSubmatch(string(body), -1)
	if len(refs) < 2 {
		t.Fatalf("index.html references %d bundle assets, want a script and a stylesheet: %.200s", len(refs), body)
	}
	for _, ref := range refs {
		res, err := http.Get(server.URL + "/" + ref[1])
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusOK || strings.HasPrefix(res.Header.Get("Content-Type"), "text/html") {
			t.Errorf("/%s: status %d, content type %q", ref[1], res.StatusCode, res.Header.Get("Content-Type"))
		}
	}
}

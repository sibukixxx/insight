package openapigen

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const repoRoot = "../.."

var specFiles = []string{PublicSpecPath, "docs/openapi/reference-api.json"}

func readRepoFile(t *testing.T, rel string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot, rel))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// TestPublicSpecIsCurrent fails when the committed OpenAPI view no longer
// matches what the contract schema generates (run `make openapi`).
func TestPublicSpecIsCurrent(t *testing.T) {
	want, err := GeneratePublic(readRepoFile(t, SchemaPath))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(readRepoFile(t, PublicSpecPath), want) {
		t.Fatalf("%s is stale: run make openapi and commit it", PublicSpecPath)
	}
}

func TestGeneratePublicRejectsUnknownRefForms(t *testing.T) {
	schema := `{"title":"t","description":"d","$id":"x","x-contractVersion":"1","x-contractSchema":"s",
		"x-errorCodes":{},"x-limits":{},"x-operations":{},
		"$defs":{"A":{"$ref":"other.json#/$defs/B"}}}`
	if _, err := GeneratePublic([]byte(schema)); err == nil || !strings.Contains(err.Error(), "unsupported $ref") {
		t.Fatalf("want unsupported $ref error, got %v", err)
	}
}

func TestGeneratePublicRejectsOperationsNamingMissingDefs(t *testing.T) {
	schema := `{"title":"t","description":"d","$id":"x","x-contractVersion":"1","x-contractSchema":"s",
		"x-errorCodes":{},"x-limits":{},"$defs":{},
		"x-operations":{"op":{"method":"GET","path":"/a","response":"Missing","status":200}}}`
	if _, err := GeneratePublic([]byte(schema)); err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("want missing $def error, got %v", err)
	}
}

var (
	templateParam = regexp.MustCompile(`\{([^}]+)\}`)
	httpMethods   = map[string]bool{"get": true, "put": true, "post": true, "delete": true, "options": true, "head": true, "patch": true, "trace": true}
)

// TestSpecsAreStructurallyValid checks the OpenAPI 3.1 invariants a parser
// would reject, without an external dependency: version, info, resolvable
// internal $refs only, unique operationIds, path templates and declared path
// parameters agreeing, and at least one response per operation.
func TestSpecsAreStructurallyValid(t *testing.T) {
	for _, rel := range specFiles {
		t.Run(filepath.Base(rel), func(t *testing.T) {
			var doc map[string]any
			dec := json.NewDecoder(bytes.NewReader(readRepoFile(t, rel)))
			dec.UseNumber()
			if err := dec.Decode(&doc); err != nil {
				t.Fatal(err)
			}
			if doc["openapi"] != "3.1.0" {
				t.Errorf("openapi = %v, want 3.1.0", doc["openapi"])
			}
			info, _ := doc["info"].(map[string]any)
			if s, _ := info["title"].(string); s == "" {
				t.Error("info.title is empty")
			}
			if s, _ := info["version"].(string); s == "" {
				t.Error("info.version is empty")
			}
			walkRefs(doc, func(ref string) {
				if !strings.HasPrefix(ref, "#/") {
					t.Errorf("external $ref %q: specs must be self-contained", ref)
					return
				}
				if _, ok := resolvePointer(doc, ref); !ok {
					t.Errorf("$ref %q does not resolve", ref)
				}
			})
			paths, ok := doc["paths"].(map[string]any)
			if !ok || len(paths) == 0 {
				t.Fatal("no paths")
			}
			seen := map[string]string{}
			for path, item := range paths {
				ops := item.(map[string]any)
				pathLevel := declaredPathParams(ops["parameters"])
				for method, raw := range ops {
					if method == "parameters" || method == "summary" || method == "description" {
						continue
					}
					if !httpMethods[method] {
						t.Errorf("%s: unknown key %q", path, method)
						continue
					}
					op := raw.(map[string]any)
					id, _ := op["operationId"].(string)
					if id == "" {
						t.Errorf("%s %s: missing operationId", method, path)
					} else if prev, dup := seen[id]; dup {
						t.Errorf("operationId %s used by %s and %s %s", id, prev, method, path)
					}
					seen[id] = method + " " + path
					declared := declaredPathParams(op["parameters"])
					for k := range pathLevel {
						declared[k] = true
					}
					var inTemplate []string
					for _, m := range templateParam.FindAllStringSubmatch(path, -1) {
						inTemplate = append(inTemplate, m[1])
						if !declared[m[1]] {
							t.Errorf("%s %s: path parameter %s is not declared (in: path, required: true)", method, path, m[1])
						}
					}
					if len(declared) != len(inTemplate) {
						t.Errorf("%s %s: declares path parameters %v, template has %v", method, path, keys(declared), inTemplate)
					}
					responses, _ := op["responses"].(map[string]any)
					if len(responses) == 0 {
						t.Errorf("%s %s: no responses", method, path)
					}
					for code := range responses {
						if code == "default" {
							continue
						}
						if n, err := strconv.Atoi(code); err != nil || n < 100 || n > 599 {
							t.Errorf("%s %s: invalid response code %q", method, path, code)
						}
					}
				}
			}
		})
	}
}

func declaredPathParams(raw any) map[string]bool {
	out := map[string]bool{}
	list, _ := raw.([]any)
	for _, p := range list {
		param, _ := p.(map[string]any)
		if param["in"] == "path" && param["required"] == true {
			if name, _ := param["name"].(string); name != "" {
				out[name] = true
			}
		}
	}
	return out
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func walkRefs(node any, visit func(string)) {
	switch v := node.(type) {
	case map[string]any:
		for k, child := range v {
			if k == "$ref" {
				if s, ok := child.(string); ok {
					visit(s)
				}
				continue
			}
			walkRefs(child, visit)
		}
	case []any:
		for _, child := range v {
			walkRefs(child, visit)
		}
	}
}

func resolvePointer(doc any, ref string) (any, bool) {
	cur := doc
	for _, token := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
		token = strings.ReplaceAll(strings.ReplaceAll(token, "~1", "/"), "~0", "~")
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		if cur, ok = m[token]; !ok {
			return nil, false
		}
	}
	return cur, true
}

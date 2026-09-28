package httpapi_test

import (
	"encoding/json"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	httpapi "insight-lab/internal/http"
	"insight-lab/internal/publicengine"
)

const (
	publicPrefix      = "/api/public/v1"
	publicSpecFile    = "../../docs/openapi/public-engine-v1.json"
	referenceSpecFile = "../../docs/openapi/reference-api.json"
	contractSchema    = "../../contracts/public-engine/v1/schema.json"
)

var anyParam = regexp.MustCompile(`\{[^}]+\}`)

// routeKey normalizes "METHOD /path/{name}" so that parameter spelling
// ({subjectID} in chi, {subjectId} in the contract) and the trailing slash
// chi reports for r.Get("/") inside r.Route do not count as drift.
func routeKey(method, path string) string {
	if len(path) > 1 {
		path = strings.TrimSuffix(path, "/")
	}
	return strings.ToUpper(method) + " " + anyParam.ReplaceAllString(path, "{}")
}

// servedRoutes walks the real router built by NewRouter. A zero Engine is
// enough to mount /api/public/v1: public.Router never dereferences it while
// registering routes.
func servedRoutes(t *testing.T, withPublic bool) map[string]string {
	t.Helper()
	deps := httpapi.Deps{NoWeb: true}
	if withPublic {
		deps.PublicEngine = &publicengine.Engine{}
	}
	routes, ok := httpapi.NewRouter(deps).(chi.Routes)
	if !ok {
		t.Fatal("NewRouter no longer returns a chi router; update the drift test walker")
	}
	out := map[string]string{}
	err := chi.Walk(routes, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		out[routeKey(method, route)] = method + " " + route
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

type specOperation struct {
	OperationID string `json:"operationId"`
	Build       string `json:"x-build"`
}

type specDoc struct {
	Paths    map[string]map[string]json.RawMessage `json:"paths"`
	Coverage struct {
		Covered       []string `json:"covered"`
		NotYetCovered []string `json:"notYetCovered"`
	} `json:"x-coverage"`
}

func loadSpec(t *testing.T, file string) (specDoc, map[string]specOperation) {
	t.Helper()
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var doc specDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	ops := map[string]specOperation{}
	for path, item := range doc.Paths {
		for method, raw := range item {
			if method == "parameters" || method == "summary" || method == "description" {
				continue
			}
			var op specOperation
			if err := json.Unmarshal(raw, &op); err != nil {
				t.Fatalf("%s %s: %v", method, path, err)
			}
			ops[routeKey(method, path)] = op
		}
	}
	return doc, ops
}

func diffKeys(t *testing.T, label string, got, want map[string]bool) {
	t.Helper()
	var missing, extra []string
	for k := range want {
		if !got[k] {
			missing = append(missing, k)
		}
	}
	for k := range got {
		if !want[k] {
			extra = append(extra, k)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	if len(missing) > 0 || len(extra) > 0 {
		t.Errorf("%s:\n  missing: %v\n  unexpected: %v", label, missing, extra)
	}
}

// TestPublicOpenAPIMatchesRouterAndContract keeps three views of the Public
// Engine identical: the routes the real router serves under /api/public/v1,
// the schema's x-operations, and the operations in the generated OpenAPI
// document (with operationId equal to the x-operations key). A capability
// the schema lists without a served route fails here instead of being
// advertised as an HTTP operation.
func TestPublicOpenAPIMatchesRouterAndContract(t *testing.T) {
	router := map[string]bool{}
	for key := range servedRoutes(t, true) {
		if strings.Contains(key, " "+publicPrefix+"/") {
			router[key] = true
		}
	}
	if len(router) == 0 {
		t.Fatal("no routes found under " + publicPrefix + "; is the public router still mounted there?")
	}

	data, err := os.ReadFile(contractSchema)
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Operations map[string]struct {
			Method string `json:"method"`
			Path   string `json:"path"`
		} `json:"x-operations"`
	}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	contract, contractIDs := map[string]bool{}, map[string]string{}
	for id, op := range schema.Operations {
		key := routeKey(op.Method, op.Path)
		contract[key] = true
		contractIDs[key] = id
	}

	_, ops := loadSpec(t, publicSpecFile)
	spec := map[string]bool{}
	for key, op := range ops {
		spec[key] = true
		if want := contractIDs[key]; want != "" && op.OperationID != want {
			t.Errorf("%s: operationId %q, x-operations key %q", key, op.OperationID, want)
		}
	}

	diffKeys(t, "router vs schema x-operations", router, contract)
	diffKeys(t, "OpenAPI vs schema x-operations", spec, contract)
}

// TestReferenceOpenAPICoverageMatchesRouter makes the Reference API's
// x-coverage an exact inventory of router.go (outside /api/public/v1):
// covered + notYetCovered = served routes, disjoint, and covered = the
// operations the document describes. Demo routes carry x-build: demo.
func TestReferenceOpenAPICoverageMatchesRouter(t *testing.T) {
	router := map[string]bool{}
	for key := range servedRoutes(t, false) {
		if strings.Contains(key, " "+publicPrefix+"/") {
			t.Fatalf("public route %s served without a PublicEngine", key)
		}
		router[key] = true
	}

	doc, ops := loadSpec(t, referenceSpecFile)
	covered, inventory := map[string]bool{}, map[string]bool{}
	add := func(list []string, into map[string]bool, label string) {
		for _, entry := range list {
			method, path, ok := strings.Cut(entry, " ")
			if !ok {
				t.Errorf("x-coverage.%s entry %q is not METHOD /path", label, entry)
				continue
			}
			key := routeKey(method, path)
			if inventory[key] {
				t.Errorf("x-coverage lists %s twice", entry)
			}
			inventory[key] = true
			into[key] = true
		}
	}
	add(doc.Coverage.Covered, covered, "covered")
	add(doc.Coverage.NotYetCovered, map[string]bool{}, "notYetCovered")

	spec := map[string]bool{}
	for key, op := range ops {
		spec[key] = true
		isDemo := strings.Contains(key, " /api/demo")
		if isDemo && op.Build != "demo" {
			t.Errorf("%s: demo route without x-build: demo", key)
		}
		if !isDemo && op.Build != "" {
			t.Errorf("%s: unexpected x-build %q", key, op.Build)
		}
	}

	diffKeys(t, "x-coverage (covered+notYetCovered) vs router.go", inventory, router)
	diffKeys(t, "described operations vs x-coverage.covered", spec, covered)
}

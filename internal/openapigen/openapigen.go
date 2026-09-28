// Package openapigen derives the OpenAPI 3.1 description of the Public
// Engine Contract v1 from contracts/public-engine/v1/schema.json. The JSON
// Schema stays the single source of truth: operations come from its
// x-operations, component schemas are its $defs with only the $ref prefix
// rewritten, and nothing is described that the schema does not list.
package openapigen

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	// SchemaPath is the contract schema, relative to the repository root.
	SchemaPath = "contracts/public-engine/v1/schema.json"
	// PublicSpecPath is the generated spec, relative to the repository root.
	PublicSpecPath = "docs/openapi/public-engine-v1.json"

	defsPrefix       = "#/$defs/"
	componentsPrefix = "#/components/schemas/"
)

var pathParam = regexp.MustCompile(`\{([^}]+)\}`)

type operation struct {
	Method   string `json:"method"`
	Path     string `json:"path"`
	Request  string `json:"request"`
	Response string `json:"response"`
	Status   int    `json:"status"`
}

// GeneratePublic converts the contract schema into the OpenAPI 3.1 document
// committed at PublicSpecPath. The output is deterministic (map keys are
// sorted by encoding/json) and ends with a newline.
func GeneratePublic(schema []byte) ([]byte, error) {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(schema, &root); err != nil {
		return nil, fmt.Errorf("parse schema: %w", err)
	}
	var (
		title, description, version, contractSchema, id string
		ops                                             map[string]operation
		defs                                            map[string]any
		errorCodes                                      map[string]int
		limits                                          map[string]any
	)
	fields := []struct {
		key string
		dst any
	}{
		{"title", &title}, {"description", &description}, {"$id", &id},
		{"x-contractVersion", &version}, {"x-contractSchema", &contractSchema},
		{"x-operations", &ops}, {"$defs", &defs}, {"x-errorCodes", &errorCodes}, {"x-limits", &limits},
	}
	for _, f := range fields {
		raw, ok := root[f.key]
		if !ok {
			return nil, fmt.Errorf("schema has no %s", f.key)
		}
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.UseNumber()
		if err := dec.Decode(f.dst); err != nil {
			return nil, fmt.Errorf("parse schema %s: %w", f.key, err)
		}
	}

	schemas := map[string]any{}
	for name, def := range defs {
		rewritten, err := rewriteRefs(def)
		if err != nil {
			return nil, fmt.Errorf("$defs/%s: %w", name, err)
		}
		schemas[name] = rewritten
	}

	paths := map[string]map[string]any{}
	for opID, op := range ops {
		if err := checkRef(defs, op.Response); err != nil {
			return nil, fmt.Errorf("x-operations.%s response: %w", opID, err)
		}
		if op.Request != "" {
			if err := checkRef(defs, op.Request); err != nil {
				return nil, fmt.Errorf("x-operations.%s request: %w", opID, err)
			}
		}
		method := strings.ToLower(op.Method)
		if paths[op.Path] == nil {
			paths[op.Path] = map[string]any{}
		}
		if _, dup := paths[op.Path][method]; dup {
			return nil, fmt.Errorf("x-operations.%s duplicates %s %s", opID, op.Method, op.Path)
		}
		paths[op.Path][method] = buildOperation(opID, op, errorCodes)
	}

	doc := map[string]any{
		"openapi":           "3.1.0",
		"jsonSchemaDialect": "https://json-schema.org/draft/2020-12/schema",
		"info": map[string]any{
			"title":   title + " (OpenAPI view)",
			"version": version,
			"summary": "Public Engine Contract v1 over HTTP/JSON under /api/public/v1.",
			"license": map[string]any{"name": "Apache-2.0", "identifier": "Apache-2.0"},
			"description": "GENERATED from " + SchemaPath + " by `make openapi` (cmd/insight-openapi); do not edit by hand. " +
				"The JSON Schema is authoritative: operations are its x-operations and components.schemas are its $defs with " +
				"only the $ref prefix rewritten. This is the public, SDK-backed contract; the Reference API " +
				"(docs/openapi/reference-api.json) is a separate, non-stable surface.\n\n" + description,
		},
		"servers": []any{map[string]any{
			"url":         "/",
			"description": "Same origin as the page. Paths carry the full /api/public/v1 prefix.",
		}},
		// The transport has no authentication (the server binds to loopback
		// and rejects non-loopback browser origins); say so explicitly.
		"security":          []any{},
		"paths":             paths,
		"components":        map[string]any{"schemas": schemas},
		"x-source":          map[string]any{"schema": SchemaPath, "schemaId": id},
		"x-contractSchema":  contractSchema,
		"x-contractVersion": version,
		"x-errorCodes":      errorCodes,
		"x-limits":          limits,
	}

	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}

// statusTable renders x-errorCodes as "400: A, B; 404: C; ..." so the error
// prose is derived from the schema and cannot drift from it.
func statusTable(errorCodes map[string]int, only func(int) bool) string {
	byStatus := map[int][]string{}
	var statuses []int
	for code, status := range errorCodes {
		if only != nil && !only(status) {
			continue
		}
		if byStatus[status] == nil {
			statuses = append(statuses, status)
		}
		byStatus[status] = append(byStatus[status], code)
	}
	sort.Ints(statuses)
	parts := make([]string, 0, len(statuses))
	for _, status := range statuses {
		codes := byStatus[status]
		sort.Strings(codes)
		parts = append(parts, strconv.Itoa(status)+": "+strings.Join(codes, ", "))
	}
	return strings.Join(parts, "; ")
}

func buildOperation(opID string, op operation, errorCodes map[string]int) map[string]any {
	success := map[string]any{
		"description": "Success (" + op.Response + ").",
		"content":     jsonContent(op.Response),
	}
	responses := map[string]any{
		strconv.Itoa(op.Status): success,
		"default": map[string]any{
			"description": "Contract error (ErrorResponse). The HTTP status follows x-errorCodes (" +
				statusTable(errorCodes, nil) + "). Which codes a given operation returns is not enumerated per operation. " +
				"Unknown operations answer NOT_FOUND and wrong methods INVALID_REQUEST.",
			"content": jsonContent("ErrorResponse"),
		},
	}
	out := map[string]any{
		"operationId": opID,
		"summary":     op.Method + " " + op.Path,
		"tags":        []any{"public-engine-v1"},
		"responses":   responses,
		"x-contractOperation": map[string]any{
			"request": op.Request, "response": op.Response, "status": op.Status,
		},
	}
	if params := pathParameters(op.Path); len(params) > 0 {
		out["parameters"] = params
	}
	if op.Request != "" {
		out["requestBody"] = map[string]any{
			"required":    true,
			"description": "JSON body, at most x-limits.maxRequestBytes. Unknown fields are tolerated.",
			"content":     jsonContent(op.Request),
		}
		responses["400"] = map[string]any{
			"description": "The body is not valid JSON or exceeds x-limits.maxRequestBytes (INVALID_REQUEST), " +
				"or another 400 code applies (" + statusTable(errorCodes, func(s int) bool { return s == 400 }) + ").",
			"content": jsonContent("ErrorResponse"),
		}
	}
	return out
}

func pathParameters(path string) []any {
	var params []any
	for _, m := range pathParam.FindAllStringSubmatch(path, -1) {
		params = append(params, map[string]any{
			"name": m[1], "in": "path", "required": true,
			"description": "Opaque engine-issued identifier.",
			"schema":      map[string]any{"type": "string"},
		})
	}
	return params
}

func jsonContent(def string) map[string]any {
	return map[string]any{"application/json": map[string]any{
		"schema": map[string]any{"$ref": componentsPrefix + def},
	}}
}

func checkRef(defs map[string]any, name string) error {
	if name == "" {
		return fmt.Errorf("missing $def name")
	}
	if _, ok := defs[name]; !ok {
		return fmt.Errorf("$defs/%s does not exist", name)
	}
	return nil
}

// rewriteRefs copies a schema node, pointing internal "#/$defs/X" references
// at "#/components/schemas/X". Any other reference form is rejected so a
// future schema change cannot silently produce a dangling OpenAPI $ref.
func rewriteRefs(node any) (any, error) {
	switch v := node.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if k == "$ref" {
				ref, ok := v[k].(string)
				if !ok || !strings.HasPrefix(ref, defsPrefix) {
					return nil, fmt.Errorf("unsupported $ref %v", v[k])
				}
				out[k] = componentsPrefix + strings.TrimPrefix(ref, defsPrefix)
				continue
			}
			if k == "$id" || k == "$anchor" || k == "$dynamicRef" || k == "$dynamicAnchor" {
				return nil, fmt.Errorf("unsupported keyword %s inside $defs", k)
			}
			child, err := rewriteRefs(v[k])
			if err != nil {
				return nil, err
			}
			out[k] = child
		}
		return out, nil
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			child, err := rewriteRefs(item)
			if err != nil {
				return nil, err
			}
			out[i] = child
		}
		return out, nil
	default:
		return v, nil
	}
}

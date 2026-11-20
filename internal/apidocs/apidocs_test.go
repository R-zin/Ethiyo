package apidocs

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/R-zin/Ethiyo/internal/breaker"
	"github.com/R-zin/Ethiyo/internal/models"
)

type document struct {
	OpenAPI    string                    `json:"openapi"`
	Paths      map[string]map[string]any `json:"paths"`
	Components map[string]map[string]any `json:"components"`
}

func load(t *testing.T) (document, any) {
	t.Helper()
	var doc document
	if err := json.Unmarshal(Spec(), &doc); err != nil {
		t.Fatalf("openapi.json is not valid JSON: %v", err)
	}
	var raw any
	_ = json.Unmarshal(Spec(), &raw)
	return doc, raw
}

func TestSpecBasics(t *testing.T) {
	doc, _ := load(t)
	if doc.OpenAPI != "3.1.0" {
		t.Errorf("openapi = %q, want 3.1.0", doc.OpenAPI)
	}
	if len(doc.Paths) == 0 {
		t.Fatal("no paths documented")
	}
}

// TestRefsResolve walks the document and checks every $ref points at an
// existing component.
func TestRefsResolve(t *testing.T) {
	doc, raw := load(t)
	var walk func(v any, at string)
	walk = func(v any, at string) {
		switch n := v.(type) {
		case map[string]any:
			for k, child := range n {
				if k == "$ref" {
					ref, _ := child.(string)
					parts := strings.Split(strings.TrimPrefix(ref, "#/components/"), "/")
					if !strings.HasPrefix(ref, "#/components/") || len(parts) != 2 || doc.Components[parts[0]][parts[1]] == nil {
						t.Errorf("unresolvable $ref %q at %s", ref, at)
					}
					continue
				}
				walk(child, at+"/"+k)
			}
		case []any:
			for _, child := range n {
				walk(child, at+"[]")
			}
		}
	}
	walk(raw, "#")
}

// jsonFields returns the JSON names of a struct's exported fields,
// flattening embedded structs as encoding/json does.
func jsonFields(t reflect.Type) []string {
	var names []string
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		tag := f.Tag.Get("json")
		if f.Anonymous && tag == "" {
			names = append(names, jsonFields(f.Type)...)
			continue
		}
		name := strings.Split(tag, ",")[0]
		if name == "-" || !f.IsExported() {
			continue
		}
		if name == "" {
			name = f.Name
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// TestSchemasMatchModels fails when a model gains, loses or renames a JSON
// field without the documentation following (or the other way round).
func TestSchemasMatchModels(t *testing.T) {
	doc, _ := load(t)
	cases := map[string]any{
		"ErrorDetail":     models.ErrorDetail{},
		"ErrorResponse":   models.ErrorResponse{},
		"BusLiveInfo":     models.BusLiveInfo{},
		"BusTrackingInfo": models.BusTrackingInfo{},
		"BusRouteInfo":    models.BusRouteInfo{},
		"PositionFix":     models.PositionFix{},
		"BusHistory":      models.BusHistory{},
		"FleetItem":       models.FleetItem{},
		"FleetResponse":   models.FleetResponse{},
		"UpstreamStatus":  breaker.Status{},
	}
	for name, model := range cases {
		schema, ok := doc.Components["schemas"][name].(map[string]any)
		if !ok {
			t.Errorf("schema %s missing", name)
			continue
		}
		props, _ := schema["properties"].(map[string]any)
		var documented []string
		for p := range props {
			documented = append(documented, p)
		}
		sort.Strings(documented)
		if got, want := strings.Join(documented, ","), strings.Join(jsonFields(reflect.TypeOf(model)), ","); got != want {
			t.Errorf("schema %s properties [%s] != model fields [%s]", name, got, want)
		}
	}
}

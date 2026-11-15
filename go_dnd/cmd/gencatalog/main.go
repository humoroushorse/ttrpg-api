// gencatalog writes the OpenAPI paths and route methods for internal/entities.List:
//
//	api/openapi/src/catalog.gen.yaml   (JSON, which is YAML; merged by merge-openapi.py)
//	internal/entities/routes.gen.go
//
// Run it from go_dnd (make generate does), then merge-openapi and oapi-codegen.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/format"
	"os"
	"strings"
	"text/template"

	"github.com/humoroushorse/go_dnd/internal/entities"
	"github.com/humoroushorse/go_dnd/internal/table"
)

type obj = map[string]any

func ref(name string) obj { return obj{"$ref": "#/components/" + name} }

func errorResponses(codes ...string) obj {
	out := obj{}
	for _, code := range codes {
		out[code] = ref("responses/Error")
	}
	return out
}

func jsonBody(schema string) obj {
	return obj{"required": true, "content": obj{"application/json": obj{"schema": ref("schemas/" + schema)}}}
}

func okResponse(description string, schema obj, errors ...string) obj {
	responses := errorResponses(errors...)
	responses["200"] = obj{"description": description, "content": obj{"application/json": obj{"schema": schema}}}
	return responses
}

func colSchema(c table.Col) obj {
	var s obj
	switch c.Type {
	case table.Text:
		s = obj{"type": "string"}
	case table.Int:
		s = obj{"type": "integer"}
	case table.Bool:
		s = obj{"type": "boolean"}
	case table.TextArray:
		s = obj{"type": "array", "items": obj{"type": "string"}}
	case table.JSON:
		if c.Shape == "array" {
			s = obj{"type": "array", "items": obj{}}
		} else {
			s = obj{"type": "object", "additionalProperties": true}
		}
	case table.UUID:
		s = obj{"type": "string", "format": "uuid"}
	case table.Time:
		s = obj{"type": "string", "format": "date-time"}
	}
	if c.Nullable {
		s["nullable"] = true
	}
	if len(c.Aliases) > 0 {
		s["description"] = "Also accepted as " + strings.Join(c.Aliases, ", ") + "."
	}
	return s
}

func schemas(e entities.Entity) obj {
	fields, response := obj{}, obj{}
	var required []string
	for _, c := range e.Table.Cols {
		response[c.Name] = colSchema(c)
		if !c.Server {
			fields[c.Name] = colSchema(c)
			if c.Required {
				required = append(required, c.Name)
			}
		}
	}
	response["source"] = ref("schemas/SourceRef")
	return obj{
		e.Singular + "Fields": obj{"type": "object", "description": "Every field a client writes. Unknown keys are a 422.",
			"required": required, "additionalProperties": false, "properties": fields},
		e.Singular + "Patch": obj{"type": "object", "description": "Only the fields to change (any of " + e.Singular + "Fields).",
			"additionalProperties": true},
		e.Singular: obj{"type": "object", "description": "As the API returns it. ?fields= leaves the other fields out.",
			"properties": response},
	}
}

func paths(e entities.Entity) obj {
	base := "/api/v1/" + e.Path
	tags := []string{e.Tag}
	one := ref("schemas/" + e.Singular)
	fields := ref("parameters/Fields")
	return obj{
		base + "/query": obj{"get": obj{
			"tags": tags, "operationId": "query" + e.Plural, "summary": "By name then newest edition, after sort",
			"parameters": []obj{ref("parameters/Name"), ref("parameters/Filter"), ref("parameters/Sort"),
				ref("parameters/Limit"), ref("parameters/Offset"), fields},
			"responses": okResponse("A page (fields left out by ?fields= are absent)", obj{"allOf": []obj{
				ref("schemas/GenericListResponse"),
				{"type": "object", "properties": obj{"entities": obj{"type": "array", "items": one}}},
			}}, "422"),
		}},
		base: obj{"post": obj{
			"tags": tags, "operationId": "create" + e.Singular, "summary": "Create; a (source, slug) that's taken is a 409",
			"requestBody": jsonBody(e.Singular + "Fields"),
			"responses":   okResponse("As stored", one, "401", "409", "422"),
		}},
		base + "/bulk": obj{"post": obj{
			"tags": tags, "operationId": "bulkLoad" + e.Plural,
			"summary": "Create or update from a file (an ETL _all.json), keyed on (source, slug); all or nothing",
			"requestBody": obj{"required": true, "content": obj{"multipart/form-data": obj{"schema": obj{
				"type": "object", "required": []string{"file"},
				"properties": obj{"file": obj{"type": "string", "format": "binary"}},
			}}}},
			"responses": okResponse("What happened to each row", ref("schemas/BulkLoadResponse"), "400", "401"),
		}},
		base + "/{entity_id}": obj{
			"parameters": []obj{ref("parameters/EntityId")},
			"get": obj{"tags": tags, "operationId": "read" + e.Singular, "parameters": []obj{fields},
				"responses": okResponse("One entry", one, "404", "422")},
			"put": obj{"tags": tags, "operationId": "replace" + e.Singular, "summary": "Replace; fields left out go back to their defaults",
				"requestBody": jsonBody(e.Singular + "Fields"), "responses": okResponse("As stored", one, "401", "404", "409", "422")},
			"patch": obj{"tags": tags, "operationId": "update" + e.Singular, "summary": "Change some fields; null clears an optional field",
				"requestBody": jsonBody(e.Singular + "Patch"), "responses": okResponse("As stored", one, "401", "404", "409", "422")},
			"delete": obj{"tags": tags, "operationId": "delete" + e.Singular, "summary": "Soft delete. Returns the id",
				"responses": okResponse("The id", obj{"type": "string", "format": "uuid"}, "401", "404")},
		},
		base + "/{source_slug}/{slug}": obj{"get": obj{
			"tags": tags, "operationId": "read" + e.Singular + "BySlugs",
			"summary": "One entry by its source's slug and its own (phb_2024/blinded)",
			"parameters": []obj{
				{"name": "source_slug", "in": "path", "required": true, "schema": obj{"type": "string"}},
				{"name": "slug", "in": "path", "required": true, "schema": obj{"type": "string"}},
				fields,
			},
			"responses": okResponse("One entry", one, "404", "422"),
		}},
	}
}

var routesTemplate = template.Must(template.New("routes").Parse(`// Code generated by cmd/gencatalog. DO NOT EDIT.

package entities

import (
	"net/http"

	"github.com/google/uuid"

	api "github.com/humoroushorse/go_dnd/api/generated"
	"github.com/humoroushorse/go_dnd/internal/catalog"
)

// Routes implements the generated route methods of every entity in List.
type Routes struct{ c *catalog.Handler }

// NewRoutes serves the entities through the shared catalog handler.
func NewRoutes(c *catalog.Handler) *Routes { return &Routes{c: c} }
{{range .}}{{$v := printf "List[%d]" .Index}}
func (h *Routes) Query{{.Plural}}(w http.ResponseWriter, r *http.Request, p api.Query{{.Plural}}Params) {
	h.c.Query(w, r, {{$v}}.Entity, catalog.Params{Name: p.Name, Fields: p.Fields, Filter: p.Filter, Sort: p.Sort, Limit: p.Limit, Offset: p.Offset})
}

func (h *Routes) Read{{.Singular}}(w http.ResponseWriter, r *http.Request, id api.EntityId, p api.Read{{.Singular}}Params) {
	h.c.Read(w, r, {{$v}}.Entity, uuid.UUID(id), p.Fields)
}

func (h *Routes) Read{{.Singular}}BySlugs(w http.ResponseWriter, r *http.Request, sourceSlug, slug string, p api.Read{{.Singular}}BySlugsParams) {
	h.c.ReadBySlugs(w, r, {{$v}}.Entity, sourceSlug, slug, p.Fields)
}

func (h *Routes) Create{{.Singular}}(w http.ResponseWriter, r *http.Request) { h.c.Create(w, r, {{$v}}.Entity) }

func (h *Routes) Replace{{.Singular}}(w http.ResponseWriter, r *http.Request, id api.EntityId) {
	h.c.Replace(w, r, {{$v}}.Entity, uuid.UUID(id))
}

func (h *Routes) Update{{.Singular}}(w http.ResponseWriter, r *http.Request, id api.EntityId) {
	h.c.Patch(w, r, {{$v}}.Entity, uuid.UUID(id))
}

func (h *Routes) Delete{{.Singular}}(w http.ResponseWriter, r *http.Request, id api.EntityId) {
	h.c.Delete(w, r, {{$v}}.Entity, uuid.UUID(id))
}

func (h *Routes) BulkLoad{{.Plural}}(w http.ResponseWriter, r *http.Request) { h.c.Bulk(w, r, {{$v}}.Entity) }
{{end}}`))

func main() {
	spec := obj{"tags": []obj{}, "paths": obj{}, "components": obj{"schemas": obj{}}}
	type routeData struct {
		Index            int
		Singular, Plural string
	}
	var routes []routeData
	for i, e := range entities.List {
		spec["tags"] = append(spec["tags"].([]obj), obj{"name": e.Tag})
		for k, v := range paths(e) {
			spec["paths"].(obj)[k] = v
		}
		for k, v := range schemas(e) {
			spec["components"].(obj)["schemas"].(obj)[k] = v
		}
		routes = append(routes, routeData{Index: i, Singular: e.Singular, Plural: e.Plural})
	}
	yamlOut, err := json.MarshalIndent(spec, "", "  ")
	must(err)
	header := "# Code generated by cmd/gencatalog from internal/entities. DO NOT EDIT. (JSON is valid YAML.)\n"
	must(os.WriteFile("api/openapi/src/catalog.gen.yaml", append([]byte(header), yamlOut...), 0o644))

	var buf bytes.Buffer
	must(routesTemplate.Execute(&buf, routes))
	code, err := format.Source(buf.Bytes())
	must(err)
	must(os.WriteFile("internal/entities/routes.gen.go", code, 0o644))
	fmt.Printf("gencatalog: %d entities\n", len(routes))
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "gencatalog:", err)
		os.Exit(1)
	}
}

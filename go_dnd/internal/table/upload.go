package table

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"

	"github.com/humoroushorse/go_dnd/internal/httpx"
)

// maxUploadMemory is how much of an upload is held in memory; the rest spills to a temp file.
const maxUploadMemory = 32 << 20

// ReadUpload reads the multipart `file` of a bulk upload: a JSON list of objects (kept as parsed,
// numbers as json.Number) or a CSV with a header row (cells become csvCell, converted by column type).
func ReadUpload(r *http.Request) (string, []map[string]any, error) {
	if err := r.ParseMultipartForm(maxUploadMemory); err != nil {
		return "", nil, httpx.BadRequest("Expected a multipart form with a `file` field: %v", err)
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		return "", nil, httpx.Unprocessable("file: field required")
	}
	defer file.Close()
	contents, err := io.ReadAll(file)
	if err != nil {
		return "", nil, err
	}
	contentType, _, _ := mime.ParseMediaType(header.Header.Get("Content-Type"))
	switch contentType {
	case "application/json":
		rows, err := decodeJSONRows(contents)
		return header.Filename, rows, err
	case "text/csv":
		rows, err := decodeCSVRows(contents)
		return header.Filename, rows, err
	}
	return "", nil, httpx.BadRequest(
		"Invalid document type. Expected one of: ['application/json', 'text/csv'], Received: %s", header.Header.Get("Content-Type"))
}

func decodeJSONRows(contents []byte) ([]map[string]any, error) {
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.UseNumber()
	var data any
	if err := decoder.Decode(&data); err != nil {
		return nil, httpx.BadRequest("Invalid JSON: %v", err)
	}
	items, ok := data.([]any)
	if !ok {
		return nil, httpx.BadRequest("Expected a JSON list of objects.")
	}
	rows := make([]map[string]any, len(items))
	for i, item := range items {
		row, ok := item.(map[string]any)
		if !ok {
			return nil, httpx.BadRequest("Expected a JSON list of objects.")
		}
		rows[i] = row
	}
	return rows, nil
}

func decodeCSVRows(contents []byte) ([]map[string]any, error) {
	records, err := csv.NewReader(bytes.NewReader(contents)).ReadAll()
	if err != nil {
		return nil, httpx.BadRequest("Invalid CSV: %v", err)
	}
	if len(records) == 0 {
		return nil, httpx.BadRequest("Invalid CSV: %v", errors.New("no header row"))
	}
	header := records[0]
	rows := make([]map[string]any, 0, len(records)-1)
	for _, record := range records[1:] {
		row := make(map[string]any, len(header))
		for i, name := range header {
			if i < len(record) {
				row[name] = csvCell(record[i])
			}
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// BodyDoc decodes a JSON request body into a document for Normalize (numbers as json.Number).
func BodyDoc(r *http.Request) (map[string]any, error) {
	decoder := json.NewDecoder(r.Body)
	decoder.UseNumber()
	var doc map[string]any
	if err := decoder.Decode(&doc); err != nil {
		return nil, httpx.Unprocessable("The body must be a JSON object: %v", err)
	}
	return doc, nil
}

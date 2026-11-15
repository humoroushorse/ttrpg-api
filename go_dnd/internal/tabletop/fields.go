package tabletop

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/humoroushorse/go_dnd/internal/httpx"
)

// Request bodies are read as raw fields, so a PATCH can tell "left out" from "set to null".

type fields map[string]json.RawMessage

func readFields(r *http.Request, allowed ...string) (fields, error) {
	var f fields
	decoder := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 64<<10))
	if err := decoder.Decode(&f); err != nil || f == nil {
		return nil, httpx.Unprocessable("The body must be a JSON object")
	}
	var unknown []string
	for key := range f {
		if !slices.Contains(allowed, key) {
			unknown = append(unknown, key)
		}
	}
	if len(unknown) > 0 {
		slices.Sort(unknown)
		return nil, httpx.Unprocessable("%s: extra fields not permitted", strings.Join(unknown, ", "))
	}
	return f, nil
}

func isNull(raw json.RawMessage) bool { return bytes.Equal(bytes.TrimSpace(raw), []byte("null")) }

func jsonMarshal(v any) (json.RawMessage, error) { return json.Marshal(v) }

func (f fields) has(key string) bool { _, ok := f[key]; return ok }

// decode reads f[key] into dst when present. Present and null is an error unless nullable (then dst,
// a pointer to a pointer, becomes nil).
func (f fields) decode(key string, dst any, nullable bool) (bool, error) {
	raw, ok := f[key]
	if !ok {
		return false, nil
	}
	if isNull(raw) && !nullable {
		return true, httpx.Unprocessable("%s can't be null", key)
	}
	// null into a pointer sets it to nil, which is how a PATCH clears a field
	if err := json.Unmarshal(raw, dst); err != nil {
		return true, httpx.Unprocessable("%s: %v", key, err)
	}
	return true, nil
}

// text reads a trimmed string. A null clears it when nullable; empty is allowed only when allowEmpty.
func (f fields) text(key string, max int, nullable, allowEmpty bool) (*string, bool, error) {
	var s *string
	present, err := f.decode(key, &s, nullable)
	if err != nil || !present || s == nil {
		return nil, present, err
	}
	trimmed := strings.TrimSpace(*s)
	if trimmed == "" && !allowEmpty {
		return nil, true, httpx.Unprocessable("%s can't be empty", key)
	}
	if len([]rune(trimmed)) > max {
		return nil, true, httpx.Unprocessable("%s: at most %d characters", key, max)
	}
	return &trimmed, true, nil
}

func finite(key string, v float64, min, max float64) error {
	if math.IsNaN(v) || math.IsInf(v, 0) || v < min || v > max {
		return httpx.Unprocessable("%s must be between %g and %g", key, min, max)
	}
	return nil
}

func oneOf(key, v string, allowed ...string) error {
	if !slices.Contains(allowed, v) {
		return httpx.Unprocessable("%s must be one of %s", key, strings.Join(allowed, ", "))
	}
	return nil
}

// imageURL accepts absolute http(s) URLs only: every player's browser loads it.
func imageURL(key, raw string) error {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || len(raw) > 2048 {
		return httpx.Unprocessable("%s must be an http(s) URL of at most 2048 characters", key)
	}
	return nil
}

// BackgroundSource is one copy of a scene's map.
type BackgroundSource struct {
	URL  string `json:"url"`
	Type string `json:"type,omitempty"`
}

var imageTypes = []string{"image/avif", "image/webp", "image/png", "image/jpeg", "image/gif"}

func (f fields) backgroundSources(current []byte) ([]byte, error) {
	raw, ok := f["background_sources"]
	if !ok {
		return current, nil
	}
	var sources []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &sources); err != nil || sources == nil {
		return nil, httpx.Unprocessable("background_sources must be a list")
	}
	if len(sources) > 5 {
		return nil, httpx.Unprocessable("background_sources: at most 5")
	}
	out := make([]BackgroundSource, len(sources))
	for i, s := range sources {
		for key := range s {
			if key != "url" && key != "type" {
				return nil, httpx.Unprocessable("background_sources[%d].%s: extra field not permitted", i, key)
			}
		}
		if json.Unmarshal(s["url"], &out[i].URL) != nil {
			return nil, httpx.Unprocessable("background_sources[%d].url is required", i)
		}
		if err := imageURL("background_sources.url", out[i].URL); err != nil {
			return nil, err
		}
		if t, ok := s["type"]; ok && !isNull(t) {
			if json.Unmarshal(t, &out[i].Type) != nil || !slices.Contains(imageTypes, out[i].Type) {
				return nil, httpx.Unprocessable("background_sources[%d].type must be one of %s", i, strings.Join(imageTypes, ", "))
			}
		}
	}
	return json.Marshal(out)
}

// SceneSettings are a scene's presentation settings, with defaults filled in.
type SceneSettings struct {
	// numbers: a hostile marker shows players only its number unless the DM names it for them
	TokenReveal string  `json:"token_reveal"`
	GridColor   string  `json:"grid_color"`
	GridOpacity float64 `json:"grid_opacity"`
}

func defaultSettings() SceneSettings {
	return SceneSettings{TokenReveal: "numbers", GridColor: "#000000", GridOpacity: 0.25}
}

// parseSettings reads stored settings; a missing or unknown value is its default.
func parseSettings(raw []byte) SceneSettings {
	out := defaultSettings()
	var stored struct {
		TokenReveal *string  `json:"token_reveal"`
		GridColor   *string  `json:"grid_color"`
		GridOpacity *float64 `json:"grid_opacity"`
	}
	if json.Unmarshal(raw, &stored) != nil {
		return out
	}
	if stored.TokenReveal != nil && *stored.TokenReveal == "names" {
		out.TokenReveal = "names"
	}
	if stored.GridColor != nil && hexColor.MatchString(*stored.GridColor) {
		out.GridColor = *stored.GridColor
	}
	if stored.GridOpacity != nil && *stored.GridOpacity >= 0 && *stored.GridOpacity <= 1 {
		out.GridOpacity = *stored.GridOpacity
	}
	return out
}

var hexColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// settings merges the body's settings into current's: only the keys sent change.
func (f fields) settings(current []byte) ([]byte, error) {
	raw, ok := f["settings"]
	if !ok {
		if current == nil {
			return []byte(`{}`), nil
		}
		return current, nil
	}
	var in map[string]json.RawMessage
	if err := json.Unmarshal(raw, &in); err != nil || in == nil {
		return nil, httpx.Unprocessable("settings must be an object")
	}
	merged := map[string]json.RawMessage{}
	_ = json.Unmarshal(current, &merged)
	if merged == nil {
		merged = map[string]json.RawMessage{}
	}
	for key, value := range in {
		var err error
		switch key {
		case "token_reveal":
			var s string
			if json.Unmarshal(value, &s) != nil || (s != "numbers" && s != "names") {
				err = httpx.Unprocessable("settings.token_reveal must be numbers or names")
			}
		case "grid_color":
			var s string
			if json.Unmarshal(value, &s) != nil || !hexColor.MatchString(s) {
				err = httpx.Unprocessable("settings.grid_color must be a color like #1a2b3c")
			}
		case "grid_opacity":
			var n float64
			if json.Unmarshal(value, &n) != nil || n < 0 || n > 1 {
				err = httpx.Unprocessable("settings.grid_opacity must be between 0 and 1")
			}
		default:
			err = httpx.Unprocessable("settings.%s: extra field not permitted", key)
		}
		if err != nil {
			return nil, err
		}
		merged[key] = value
	}
	return json.Marshal(merged)
}

// constraintError turns a violated constraint into the answer the caller can act on.
func constraintError(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}
	switch pgErr.ConstraintName {
	case "uq_scene_token_scene_id_character_id":
		return httpx.Conflict("that character already has a token in this scene")
	case "uq_scene_token_scene_id_number":
		return httpx.Conflict("another token in this scene already has that number")
	case "fk_scene_token_character", "fk_scene_token_controller_character":
		return httpx.Unprocessable("that character isn't in this campaign")
	}
	switch pgErr.Code {
	case "23505":
		return httpx.Conflict("%s", pgErr.Detail)
	case "23503", "23514":
		return httpx.Unprocessable("%s", pgErr.Message)
	}
	return err
}

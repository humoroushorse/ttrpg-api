package models

import "log/slog"

type Source struct {
	Bookkeeping
	ID             string `json:"id"`
	Name           string `json:"name"`
	NameShort      string `json:"name_short"`
	PublishYear    *int   `json:"publish_year,omitempty"`
	DndVersion     string `json:"dnd_version"`
	DndVersionYear int    `json:"dnd_version_year"`
}

func (s Source) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("id", s.ID),
		slog.String("name", s.Name),
		slog.String("dnd_version", s.DndVersion),
	)
}

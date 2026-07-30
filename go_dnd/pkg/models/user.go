package models

import "log/slog"

// DndUser is the app-level profile; auth identity lives in go_auth.
type DndUser struct {
	ID                string  `json:"id"`
	Username          *string `json:"username,omitempty"`
	ProfilePictureURL *string `json:"profile_picture_url,omitempty"`
}

func (u DndUser) LogValue() slog.Value {
	attrs := []slog.Attr{slog.String("id", u.ID)}
	if u.Username != nil {
		attrs = append(attrs, slog.String("username", *u.Username))
	}
	return slog.GroupValue(attrs...)
}

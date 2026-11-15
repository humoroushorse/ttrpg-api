// Package tabletop serves /api/v1/vtt: scenes, tokens, commands and the campaign's live event stream
// (docs/vtt). Every saved change is an event in dnd_vtt.event, numbered per campaign; commands append
// it in the same transaction as the rows it changes, and the pod publishes it after commit. Each viewer
// gets their own copy (view.go), so nothing a player may not see reaches their browser.
package tabletop

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	db "github.com/humoroushorse/go_dnd/internal/repository/tabletop"
)

// Event types.
const (
	SceneCreated    = "scene.created"
	SceneUpdated    = "scene.updated"
	SceneTrashed    = "scene.trashed"
	SceneActivated  = "scene.activated"
	MemberMoved     = "member.moved"
	MemberUpdated   = "member.updated"
	MemberLeft      = "member.left"
	TokenAdded      = "token.added"
	TokenUpdated    = "token.updated"
	TokenMoved      = "token.moved"
	TokenRemoved    = "token.removed"
	CampaignDeleted = "campaign.deleted"
	// live, never saved
	Ping = "ping"
	// sent by the stream, not saved: load the state again and reconnect from its seq
	Reload = "reload"
)

// Who a saved event is for, before the per-viewer rules in view.go.
const (
	AudienceAll = "all"
	AudienceDMs = "dms"
)

// Event is a table event as the log holds it: the full payload, before any viewer's filtering. Seq is
// 0 for a live message (a ping).
type Event struct {
	CampaignID uuid.UUID       `json:"campaign_id"`
	Seq        int64           `json:"seq"`
	SceneID    *uuid.UUID      `json:"scene_id"`
	Type       string          `json:"type"`
	Audience   string          `json:"audience"`
	Payload    json.RawMessage `json:"payload"`
	ActorID    *uuid.UUID      `json:"actor_id"`
	CreatedAt  time.Time       `json:"created_at"`
}

func eventFromRow(campaignID uuid.UUID, row db.EventsAfterRow) Event {
	return Event{CampaignID: campaignID, Seq: row.Seq, SceneID: row.SceneID, Type: row.Type, Audience: row.Audience,
		Payload: row.Payload, ActorID: row.CreatedBy, CreatedAt: row.CreatedAt.Time}
}

// TokenRecord is a token as events carry it: everything, including what only DMs see, and what a
// character token takes from the sheet.
type TokenRecord struct {
	ID                    uuid.UUID  `json:"id"`
	SceneID               uuid.UUID  `json:"scene_id"`
	Kind                  string     `json:"kind"`
	CharacterID           *uuid.UUID `json:"character_id"`
	ControllerCharacterID *uuid.UUID `json:"controller_character_id"`
	// the true name; null on a character token that uses the sheet's
	Name *string `json:"name"`
	// the sheet's name, on a character token
	CharacterName     *string `json:"character_name"`
	PlayerName        *string `json:"player_name"`
	Number            *int32  `json:"number"`
	Disposition       string  `json:"disposition"`
	PlayerDisposition *string `json:"player_disposition"`
	ImageURL          *string `json:"image_url"`
	CharacterPortrait string  `json:"character_portrait,omitempty"`
	X                 float64 `json:"x"`
	Y                 float64 `json:"y"`
	SizeCells         float64 `json:"size_cells"`
	Layer             string  `json:"layer"`
}

func tokenRecord(r db.SceneTokenRow) TokenRecord {
	return TokenRecord{ID: r.ID, SceneID: r.SceneID, Kind: r.Kind, CharacterID: r.CharacterID,
		ControllerCharacterID: r.ControllerCharacterID, Name: r.Name, CharacterName: r.CharacterName,
		PlayerName: r.PlayerName, Number: r.Number, Disposition: r.Disposition, PlayerDisposition: r.PlayerDisposition,
		ImageURL: r.ImageUrl, CharacterPortrait: r.CharacterPortrait, X: r.X, Y: r.Y, SizeCells: r.SizeCells, Layer: r.Layer}
}

func tokenRecordFromList(r db.SceneTokensRow) TokenRecord { return tokenRecord(db.SceneTokenRow(r)) }

// Payloads, as stored.
type (
	tokenPayload struct {
		Token TokenRecord `json:"token"`
		// on token.updated: the layer before, so players can be told it appeared or vanished
		PreviousLayer string `json:"previous_layer,omitempty"`
	}
	movePayload struct {
		TokenID uuid.UUID `json:"token_id"`
		X       float64   `json:"x"`
		Y       float64   `json:"y"`
	}
	removePayload struct {
		TokenID uuid.UUID `json:"token_id"`
		Layer   string    `json:"layer"`
	}
	scenePayload struct {
		Scene SceneView `json:"scene"`
	}
	trashPayload struct {
		SceneIDs []uuid.UUID `json:"scene_ids"`
	}
	activatedPayload struct {
		SceneID         *uuid.UUID  `json:"scene_id"`
		PreviousSceneID *uuid.UUID  `json:"previous_scene_id,omitempty"`
		MovedBack       []uuid.UUID `json:"moved_back,omitempty"`
	}
	memberMovedPayload struct {
		UserID uuid.UUID `json:"user_id"`
		// null: back to the active scene
		SceneID *uuid.UUID `json:"scene_id"`
	}
	memberPayload struct {
		UserID uuid.UUID `json:"user_id"`
		Role   string    `json:"role,omitempty"`
	}
	pingPayload struct {
		X      float64   `json:"x"`
		Y      float64   `json:"y"`
		UserID uuid.UUID `json:"user_id"`
	}
)

// layerAudience: changes to DM-layer tokens go to DMs only.
func layerAudience(layers ...string) string {
	for _, l := range layers {
		if l != "dm" {
			return AudienceAll
		}
	}
	return AudienceDMs
}

package access

import (
	"github.com/google/uuid"

	"github.com/humoroushorse/go_dnd/internal/httpx"
)

// Who may see which scene and move which token at a campaign's table. Campaign.Can covers the rest
// (ViewTable, ManageScenes, PlaceTokens). What a viewer sees of a token they may see (its true name,
// its disposition) is decided by internal/tabletop.

// SceneVisible reports whether the caller may see a scene of this campaign: DMs any, players only
// current, the scene they're on (where a DM moved them, else the active scene; nil when neither is set).
func (c Campaign) SceneVisible(scene uuid.UUID, current *uuid.UUID) bool {
	switch c.Role {
	case DM:
		return true
	case Player:
		return current != nil && *current == scene
	}
	return false
}

// RequireScene is nil when the caller may see the scene. A player asking for another scene gets a 404
// for it, as if it didn't exist, so they can't learn what else the DM has prepared.
func (c Campaign) RequireScene(scene uuid.UUID, current *uuid.UUID) error {
	if c.SceneVisible(scene, current) {
		return nil
	}
	if c.Role == None {
		return httpx.NotFound("Campaign %s not found", c.ID)
	}
	return httpx.NotFound("Scene %s not found", scene)
}

// Token layers. Players are never sent the DM layer.
const (
	LayerTokens = "tokens"
	LayerDM     = "dm"
)

// Token is what decides who may see and move a token.
type Token struct {
	ID    uuid.UUID
	Layer string
	// the character the token is
	CharacterID *uuid.UUID
	// on a marker: the character whose player moves it (a familiar, a summon)
	ControllerCharacterID *uuid.UUID
}

// TokenVisible reports whether the caller may know the token is there: DMs always, players unless
// it's on the DM layer.
func (c Campaign) TokenVisible(t Token) bool {
	switch c.Role {
	case DM:
		return true
	case Player:
		return t.Layer != LayerDM
	}
	return false
}

// CharacterFacts is what EditableCharacters needs about one of the campaign's characters.
type CharacterFacts struct {
	ID      uuid.UUID
	OwnerID uuid.UUID
	// the caller is on its editor list
	Editor bool
}

// EditableCharacters is the set of characters the caller may edit (Character.Can(EditBuild)), out of
// the campaign's characters, so a whole scene's can_move comes from one query.
func EditableCharacters(userID uuid.UUID, characters []CharacterFacts) map[uuid.UUID]bool {
	out := make(map[uuid.UUID]bool, len(characters))
	for _, f := range characters {
		if CharacterFrom(f.ID, userID, f.OwnerID, f.Editor, nil, nil, nil, nil).Can(EditBuild) {
			out[f.ID] = true
		}
	}
	return out
}

// CanMoveToken reports whether the caller may move the token: DMs any; players a visible token of a
// character they may edit, or a marker such a character controls. editable comes from
// EditableCharacters.
func (c Campaign) CanMoveToken(t Token, editable map[uuid.UUID]bool) bool {
	switch {
	case c.Role == DM:
		return true
	case !c.TokenVisible(t):
		return false
	case t.CharacterID != nil:
		return editable[*t.CharacterID]
	case t.ControllerCharacterID != nil:
		return editable[*t.ControllerCharacterID]
	}
	return false
}

// RequireMoveToken is nil when the caller may move the token, else the error to answer with. A
// DM-layer token is a 404 to players, since they mustn't learn it's there.
func (c Campaign) RequireMoveToken(t Token, editable map[uuid.UUID]bool) error {
	if c.CanMoveToken(t, editable) {
		return nil
	}
	if c.Role == None {
		return httpx.NotFound("Campaign %s not found", c.ID)
	}
	if !c.TokenVisible(t) {
		return httpx.NotFound("Token %s not found", t.ID)
	}
	return httpx.Forbidden("only the campaign's DMs and whoever controls the token can move it")
}

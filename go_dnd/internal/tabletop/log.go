package tabletop

import (
	"context"
	"encoding/json"
	"errors"
	"slices"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/humoroushorse/go_dnd/internal/repository/tabletop"
)

// Table is the table's write side, shared by its own routes and by the campaign and character routes
// whose changes reach the table (a member leaving takes their tokens). Events are appended inside the
// caller's transaction and published after it commits.
type Table struct {
	hub *Hub
}

// NewTable returns a Table that publishes through hub.
func NewTable(hub *Hub) *Table { return &Table{hub: hub} }

// Publish sends committed events to every stream. Call it only after the transaction committed.
func (t *Table) Publish(ctx context.Context, events []Event) {
	if t != nil && len(events) > 0 {
		t.hub.Publish(ctx, events)
	}
}

// lock is the first statement of every table write: it makes the campaign's state row if needed and
// locks it until the transaction ends, so the campaign's writes apply one at a time and their seqs
// commit in order.
func lock(ctx context.Context, q *db.Queries, campaignID, userID uuid.UUID) (db.LockTableStateRow, error) {
	if err := q.EnsureCampaignState(ctx, db.EnsureCampaignStateParams{CampaignID: campaignID, UserID: userID}); err != nil {
		return db.LockTableStateRow{}, err
	}
	return q.LockTableState(ctx, db.LockTableStateParams{CampaignID: campaignID, UserID: userID})
}

// draft is an event about to be appended.
type draft struct {
	typ, audience string
	sceneID       *uuid.UUID
	payload       any
}

// appendEvent takes the next seq and writes the event. lock must have run in this transaction.
func appendEvent(ctx context.Context, q *db.Queries, campaignID, by uuid.UUID, commandID *uuid.UUID, d draft) (Event, error) {
	payload, err := json.Marshal(d.payload)
	if err != nil {
		return Event{}, err
	}
	row, err := q.AppendEvent(ctx, db.AppendEventParams{CampaignID: campaignID, SceneID: d.sceneID, Type: d.typ,
		Audience: d.audience, Payload: payload, CommandID: commandID, UserID: by})
	if err != nil {
		return Event{}, err
	}
	return Event{CampaignID: campaignID, Seq: row.Seq, SceneID: d.sceneID, Type: d.typ, Audience: d.audience,
		Payload: payload, ActorID: &by, CreatedAt: row.CreatedAt.Time}, nil
}

// MemberLeaving logs, in tx, what a member leaving the campaign (or being removed) does at the table:
// their characters' tokens go, markers those characters controlled lose their controller, and their
// streams close. Call it before deleting the member and unlinking their characters (which deletes the
// tokens), then Publish the result after commit.
func (t *Table) MemberLeaving(ctx context.Context, tx pgx.Tx, campaignID, memberID, by uuid.UUID) ([]Event, error) {
	q := db.New(tx)
	if _, err := lock(ctx, q, campaignID, by); err != nil {
		return nil, err
	}
	characters, err := q.OwnerCharacterIDs(ctx, db.OwnerCharacterIDsParams{CampaignID: campaignID, UserID: memberID})
	if err != nil {
		return nil, err
	}
	events, err := charactersLeaving(ctx, q, campaignID, by, characters, true)
	if err != nil {
		return nil, err
	}
	left, err := appendEvent(ctx, q, campaignID, by, nil, draft{typ: MemberLeft, audience: AudienceAll,
		payload: memberPayload{UserID: memberID}})
	return append(events, left), err
}

// CharacterLeaving is MemberLeaving for one character taken out of the campaign.
func (t *Table) CharacterLeaving(ctx context.Context, tx pgx.Tx, campaignID, characterID, by uuid.UUID) ([]Event, error) {
	q := db.New(tx)
	if _, err := lock(ctx, q, campaignID, by); err != nil {
		return nil, err
	}
	return charactersLeaving(ctx, q, campaignID, by, []uuid.UUID{characterID}, true)
}

// CharacterDeleted logs that a (soft-)deleted character's tokens are gone from the table. They stay in
// the database, hidden, and markers it controlled keep their controller.
func (t *Table) CharacterDeleted(ctx context.Context, tx pgx.Tx, campaignID, characterID, by uuid.UUID) ([]Event, error) {
	q := db.New(tx)
	if _, err := lock(ctx, q, campaignID, by); err != nil {
		return nil, err
	}
	return charactersLeaving(ctx, q, campaignID, by, []uuid.UUID{characterID}, false)
}

func charactersLeaving(ctx context.Context, q *db.Queries, campaignID, by uuid.UUID, characterIDs []uuid.UUID,
	controllers bool) ([]Event, error) {
	if len(characterIDs) == 0 {
		return nil, nil
	}
	tokens, err := q.TokensOfCharacters(ctx, db.TokensOfCharactersParams{CampaignID: campaignID, Ids: characterIDs})
	if err != nil {
		return nil, err
	}
	var events []Event
	for _, tok := range tokens {
		sceneID := tok.SceneID
		var d draft
		switch {
		case tok.IsCharacter:
			d = draft{typ: TokenRemoved, audience: layerAudience(tok.Layer), sceneID: &sceneID,
				payload: removePayload{TokenID: tok.ID, Layer: tok.Layer}}
		case controllers:
			row, err := q.SceneToken(ctx, db.SceneTokenParams{ID: tok.ID, CampaignID: campaignID})
			if err != nil {
				return nil, err
			}
			record := tokenRecord(row)
			// the foreign key clears it when the character's link goes, later in this transaction
			record.ControllerCharacterID = nil
			d = draft{typ: TokenUpdated, audience: layerAudience(tok.Layer), sceneID: &sceneID,
				payload: tokenPayload{Token: record}}
		default:
			continue
		}
		e, err := appendEvent(ctx, q, campaignID, by, nil, d)
		if err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, nil
}

// MemberUpdated logs a member's new role; their streams reload as it.
func (t *Table) MemberUpdated(ctx context.Context, tx pgx.Tx, campaignID, memberID uuid.UUID, role string, by uuid.UUID) ([]Event, error) {
	q := db.New(tx)
	if _, err := lock(ctx, q, campaignID, by); err != nil {
		return nil, err
	}
	e, err := appendEvent(ctx, q, campaignID, by, nil, draft{typ: MemberUpdated, audience: AudienceAll,
		payload: memberPayload{UserID: memberID, Role: role}})
	return []Event{e}, err
}

// CampaignDeleted logs that the campaign is gone; every stream closes.
func (t *Table) CampaignDeleted(ctx context.Context, tx pgx.Tx, campaignID, by uuid.UUID) ([]Event, error) {
	q := db.New(tx)
	if _, err := lock(ctx, q, campaignID, by); err != nil {
		return nil, err
	}
	e, err := appendEvent(ctx, q, campaignID, by, nil, draft{typ: CampaignDeleted, audience: AudienceAll,
		payload: struct{}{}})
	return []Event{e}, err
}

// poolLog reads the log for the hub, from the primary.
type poolLog struct{ master *pgxpool.Pool }

// NewLogReader reads the event log through master.
func NewLogReader(master *pgxpool.Pool) LogReader { return poolLog{master: master} }

func (p poolLog) EventsAfter(ctx context.Context, campaignID uuid.UUID, after int64, limit int) ([]Event, error) {
	rows, err := db.New(p.master).EventsAfter(ctx, db.EventsAfterParams{CampaignID: campaignID, AfterSeq: after,
		MaxEvents: int32(limit)})
	if err != nil {
		return nil, err
	}
	out := make([]Event, len(rows))
	for i, row := range rows {
		out[i] = eventFromRow(campaignID, row)
	}
	return out, nil
}

func (p poolLog) LastSeqs(ctx context.Context, campaignIDs []uuid.UUID) (map[uuid.UUID]int64, error) {
	rows, err := db.New(p.master).CampaignSeqs(ctx, slices.Clone(campaignIDs))
	if errors.Is(err, pgx.ErrNoRows) {
		err = nil
	}
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]int64, len(rows))
	for _, row := range rows {
		out[row.CampaignID] = row.LastSeq
	}
	return out, nil
}

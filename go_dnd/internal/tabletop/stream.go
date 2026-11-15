package tabletop

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	api "github.com/humoroushorse/go_dnd/api/generated"
	"github.com/humoroushorse/go_dnd/internal/access"
	"github.com/humoroushorse/go_dnd/internal/httpx"
	db "github.com/humoroushorse/go_dnd/internal/repository/tabletop"
	"github.com/humoroushorse/go_dnd/pkg/logging"
)

const (
	// a comment this often keeps proxies from closing an idle stream (Cloudflare gives up at 100 s)
	heartbeat = 20 * time.Second
	// streams end after this (plus up to maxAgeJitter) even with a longer session, so a tab re-checks
	// its access now and then, and a deploy's reconnects spread out
	maxAge       = 30 * time.Minute
	maxAgeJitter = 5 * time.Minute
	// replaying more than this means the tab is better off loading the state again
	replayLimit = 1000
)

// StreamVttEvents is GET /vtt/campaigns/{entity_id}/events: server-sent events for the whole campaign,
// built for the caller. It replays what they missed from the log (from the primary), then follows the
// hub.
func (h *TableRoutes) StreamVttEvents(w http.ResponseWriter, r *http.Request, id api.EntityId, p api.StreamVttEventsParams) {
	ctx := r.Context()
	logger := logging.FromContext(ctx)
	userID, err := h.users.CurrentUserID(ctx)
	if err != nil {
		writeError(w, r, err)
		return
	}
	after, err := streamAfter(r, p)
	if err != nil {
		writeError(w, r, err)
		return
	}
	q := db.New(h.master)
	who, err := require(ctx, q, id, userID, access.ViewTable)
	if err != nil {
		writeError(w, r, err)
		return
	}
	st, err := tableState(ctx, q, id, userID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	characters, err := q.CampaignCharacterEditors(ctx, db.CampaignCharacterEditorsParams{CampaignID: id, UserID: userID})
	if err != nil {
		writeError(w, r, err)
		return
	}
	oldest, err := q.OldestSeq(ctx, id)
	if err != nil {
		writeError(w, r, err)
		return
	}
	viewer := NewViewer(userID, who, st.ActiveSceneID, st.MemberSceneID, characters)

	// the server's read and write timeouts would cut the stream: clear them for this response only
	rc := http.NewResponseController(w)
	for _, clear := range []func(time.Time) error{rc.SetReadDeadline, rc.SetWriteDeadline} {
		if err := clear(time.Time{}); err != nil {
			logger.Warn("table stream can't clear its deadline; it will be cut by the server timeout", "error", err)
		}
	}
	header := w.Header()
	header.Set("Content-Type", "text/event-stream")
	header.Set("Cache-Control", "no-cache, no-transform")
	header.Set("X-Accel-Buffering", "no") // nginx ingress: don't buffer
	w.WriteHeader(http.StatusOK)
	s := &sse{w: w, rc: rc}
	s.raw("retry: 3000\n\n")

	switch {
	case after > st.LastSeq:
		s.send(reloadMessage("ahead of the log"))
		return
	case oldest > 0 && after+1 < oldest:
		s.send(reloadMessage("past the log's history"))
		return
	}

	listener, err := h.table.hub.Join(ctx, id)
	if err != nil {
		return // shutting down: the tab reconnects to another pod
	}
	defer h.table.hub.Leave(id, listener)

	// replay after joining, so nothing committed in between is missed
	missed, err := db.New(h.master).EventsAfter(ctx, db.EventsAfterParams{CampaignID: id, AfterSeq: after, MaxEvents: replayLimit})
	if err != nil {
		logger.Error("table stream replay failed", "error", err)
		return
	}
	if len(missed) == replayLimit {
		s.send(reloadMessage("too far behind"))
		return
	}
	last := after
	apply := func(e Event) bool {
		if viewer.NeedsCharacters(e) {
			if rows, err := db.New(h.master).CampaignCharacterEditors(ctx,
				db.CampaignCharacterEditorsParams{CampaignID: id, UserID: userID}); err == nil {
				viewer.SetCharacters(rows)
			}
		}
		outs, effect := viewer.Apply(e)
		for _, o := range outs {
			s.send(o)
		}
		switch effect {
		case ReloadAndClose:
			s.send(reloadMessage("your role changed"))
			return false
		case Close:
			return false
		}
		return s.err == nil
	}
	for _, row := range missed {
		last = row.Seq
		if !apply(eventFromRow(id, row)) {
			return
		}
	}
	s.flush()

	age := maxAge + rand.N(maxAgeJitter)
	if expires, ok := sessionExpiry(r); ok {
		age = min(age, time.Until(expires))
	}
	end := time.NewTimer(age)
	defer end.Stop()
	beat := time.NewTicker(heartbeat)
	defer beat.Stop()
	for s.err == nil {
		select {
		case <-ctx.Done():
			return
		case <-listener.Gone():
			return
		case <-end.C:
			return // the session ended (or the stream is old): EventSource reconnects, signed in again
		case <-beat.C:
			s.raw(": keep-alive\n\n")
		case e := <-listener.Live:
			if o, ok := viewer.Live(e); ok {
				s.send(o)
			}
		case e := <-listener.Events:
			if e.Seq <= last {
				continue // replayed already
			}
			if e.Seq != last+1 {
				s.send(reloadMessage("missed events"))
				return
			}
			last = e.Seq
			if !apply(e) {
				return
			}
		}
		s.flush()
	}
}

// streamAfter is where to replay from: Last-Event-ID (a reconnect) wins over ?after= (the seq from the
// state).
func streamAfter(r *http.Request, p api.StreamVttEventsParams) (int64, error) {
	if raw := strings.TrimSpace(r.Header.Get("Last-Event-ID")); raw != "" {
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || n < 0 {
			return 0, httpx.Unprocessable("Last-Event-ID must be a seq")
		}
		return n, nil
	}
	if p.After == nil {
		return 0, httpx.Unprocessable("after: the seq from the state is required")
	}
	if *p.After < 0 {
		return 0, httpx.Unprocessable("after can't be negative")
	}
	return *p.After, nil
}

// sessionExpiry is when the caller's access token expires. The auth middleware already verified it;
// this only reads its exp.
func sessionExpiry(r *http.Request) (time.Time, bool) {
	raw := ""
	if parts := strings.SplitN(r.Header.Get("Authorization"), " ", 2); len(parts) == 2 && strings.EqualFold(parts[0], "bearer") {
		raw = parts[1]
	} else if c, err := r.Cookie("access_token"); err == nil {
		raw = c.Value
	}
	if raw == "" {
		return time.Time{}, false
	}
	var claims jwt.RegisteredClaims
	if _, _, err := jwt.NewParser().ParseUnverified(raw, &claims); err != nil || claims.ExpiresAt == nil {
		return time.Time{}, false
	}
	return claims.ExpiresAt.Time, true
}

// sse writes server-sent events, keeping the first write error.
type sse struct {
	w   http.ResponseWriter
	rc  *http.ResponseController
	err error
}

func (s *sse) raw(text string) {
	if s.err == nil {
		_, s.err = fmt.Fprint(s.w, text)
	}
}

func (s *sse) send(o Outgoing) {
	if s.err != nil {
		return
	}
	data, err := json.Marshal(o.Data)
	if err != nil {
		s.err = err
		return
	}
	if o.Seq > 0 {
		s.raw(fmt.Sprintf("id: %d\n", o.Seq))
	}
	// JSON has no raw newlines, so data is one line
	s.raw(fmt.Sprintf("event: %s\ndata: %s\n\n", o.Type, data))
	if o.Type == Reload {
		s.flush()
	}
}

func (s *sse) flush() {
	if s.err == nil {
		if err := s.rc.Flush(); err != nil && !errors.Is(err, http.ErrNotSupported) {
			s.err = err
		}
	}
}

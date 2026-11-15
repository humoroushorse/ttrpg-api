package tabletop

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/humoroushorse/go_dnd/internal/middleware"
)

// Hub is this pod's side of the table streams. For each campaign it has a stream for, it holds one bus
// subscription and the last seq it delivered, and hands every saved event to that campaign's streams
// once, in seq order, with no gaps:
//
//   - the next seq is delivered;
//   - a jump means a message was lost, so the missing events are read from the log first;
//   - a seq already delivered is dropped (the committing pod delivers locally and hears its own
//     publish again).
//
// Check runs the same gap fill on a timer, comparing each campaign's last_seq, for a pod that died
// between commit and publish or a bus that was down.
type Hub struct {
	bus    Bus
	log    LogReader
	logger *slog.Logger

	mu     sync.Mutex // guards feeds and closed; taken before any feed's mu
	feeds  map[uuid.UUID]*feed
	closed bool
}

// LogReader reads the event log, from the primary: a lagging replica would make a feed think it had
// caught up.
type LogReader interface {
	EventsAfter(ctx context.Context, campaignID uuid.UUID, after int64, limit int) ([]Event, error)
	LastSeqs(ctx context.Context, campaignIDs []uuid.UUID) (map[uuid.UUID]int64, error)
}

// ErrHubClosed is returned by Join once the pod is shutting down.
var ErrHubClosed = errors.New("tabletop hub closed")

const (
	// a stream that falls this far behind is dropped; its tab reconnects and replays from the log
	listenerBuffer = 256
	livebuffer     = 64
	fillPage       = 500
)

type feed struct {
	id      uuid.UUID
	mu      sync.Mutex
	ready   bool
	last    int64
	sub     Subscription
	early   map[int64]Event // arrived before the events in front of them
	members map[*Listener]struct{}
}

// Listener is one stream's place in a campaign's feed.
type Listener struct {
	// saved events, consecutive from the feed's last seq when the listener joined
	Events chan Event
	// pings; dropped, not queued, when the stream is slow
	Live chan Event
	gone chan struct{}
	once sync.Once
}

// Gone is closed when the hub drops the listener: the stream fell behind, or the pod is shutting down.
func (l *Listener) Gone() <-chan struct{} { return l.gone }

func (l *Listener) drop() { l.once.Do(func() { close(l.gone) }) }

// NewHub returns a hub that subscribes through bus and fills gaps from log.
func NewHub(bus Bus, log LogReader, logger *slog.Logger) *Hub {
	return &Hub{bus: bus, log: log, logger: logger, feeds: map[uuid.UUID]*feed{}}
}

// Join adds a listener to the campaign's feed, subscribing on the first one. Join before replaying
// from the log, so nothing committed in between is missed; the stream skips what it already replayed.
func (h *Hub) Join(ctx context.Context, campaignID uuid.UUID) (*Listener, error) {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return nil, ErrHubClosed
	}
	f := h.feeds[campaignID]
	if f == nil {
		f = &feed{id: campaignID, early: map[int64]Event{}, members: map[*Listener]struct{}{}}
		h.feeds[campaignID] = f
	}
	f.mu.Lock()
	h.mu.Unlock()
	if !f.ready {
		if err := h.start(ctx, f); err != nil {
			f.mu.Unlock()
			h.forget(f)
			return nil, err
		}
	}
	l := &Listener{Events: make(chan Event, listenerBuffer), Live: make(chan Event, livebuffer), gone: make(chan struct{})}
	f.members[l] = struct{}{}
	f.mu.Unlock()
	return l, nil
}

// start subscribes, then reads where the campaign's log is, so a message published in between is
// either already counted or delivered.
func (h *Hub) start(ctx context.Context, f *feed) error {
	sub, err := h.bus.Subscribe(feedSubject(f.id), func(subject string, data []byte) {
		h.receiveMessage(f.id, subject, data)
	})
	if err != nil {
		return err
	}
	seqs, err := h.log.LastSeqs(ctx, []uuid.UUID{f.id})
	if err != nil {
		_ = sub.Unsubscribe()
		return err
	}
	f.sub, f.last, f.ready = sub, seqs[f.id], true
	return nil
}

// forget removes a feed that failed to start, unless another stream joined it meanwhile.
func (h *Hub) forget(f *feed) {
	h.mu.Lock()
	defer h.mu.Unlock()
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.ready && len(f.members) == 0 && h.feeds[f.id] == f {
		delete(h.feeds, f.id)
	}
}

// Leave removes the listener, and the campaign's subscription with the last one.
func (h *Hub) Leave(campaignID uuid.UUID, l *Listener) {
	var sub Subscription
	h.mu.Lock()
	if f := h.feeds[campaignID]; f != nil {
		f.mu.Lock()
		delete(f.members, l)
		if len(f.members) == 0 {
			delete(h.feeds, campaignID)
			sub, f.sub, f.ready = f.sub, nil, false
		}
		f.mu.Unlock()
	}
	h.mu.Unlock()
	l.drop()
	if sub != nil {
		_ = sub.Unsubscribe()
	}
}

// Close drops every listener (their tabs reconnect to another pod) and stops every subscription.
func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.closed = true
	for id, f := range h.feeds {
		f.mu.Lock()
		for l := range f.members {
			l.drop()
		}
		if f.sub != nil {
			_ = f.sub.Unsubscribe()
		}
		f.ready = false
		f.mu.Unlock()
		delete(h.feeds, id)
	}
}

// envelope is a bus message (ttrpg-api/docs/nats-subjects.md).
type envelope struct {
	Timestamp time.Time  `json:"timestamp"`
	TraceID   string     `json:"trace_id"`
	ProjectID uuid.UUID  `json:"project_id"`
	UserID    *uuid.UUID `json:"user_id,omitempty"`
	Event     Event      `json:"event"`
}

// Publish hands committed events to this pod's streams, then to the other pods. Call it after the
// commit, never before: a rolled-back event must not be seen.
func (h *Hub) Publish(ctx context.Context, events []Event) {
	for _, e := range events {
		h.receive(e)
		h.send(ctx, eventSubject(e.CampaignID), e)
	}
}

// PublishLive sends a ping (never saved) to every pod's streams for the campaign.
func (h *Hub) PublishLive(ctx context.Context, e Event) {
	e.Seq = 0
	h.receive(e)
	h.send(ctx, liveSubject(e.CampaignID), e)
}

func (h *Hub) send(ctx context.Context, subject string, e Event) {
	data, err := json.Marshal(envelope{Timestamp: time.Now().UTC(), TraceID: middleware.GetTraceIDFromContext(ctx),
		ProjectID: e.CampaignID, UserID: e.ActorID, Event: e})
	if err == nil {
		err = h.bus.Publish(subject, data)
	}
	if err != nil {
		// other pods' tables catch up at their next check
		h.logger.Warn("table event not published", "subject", subject, "seq", e.Seq, "error", err)
	}
}

func (h *Hub) receiveMessage(campaignID uuid.UUID, subject string, data []byte) {
	var m envelope
	if err := json.Unmarshal(data, &m); err != nil {
		h.logger.Warn("unreadable table message", "subject", subject, "error", err)
		return
	}
	e := m.Event
	e.CampaignID = campaignID // the subject says whose it is
	switch {
	case strings.HasSuffix(subject, ".live_sent"):
		e.Seq = 0
	case strings.HasSuffix(subject, ".event_appended"):
		if e.Seq <= 0 {
			return
		}
	default:
		return
	}
	h.receive(e)
}

func (h *Hub) receive(e Event) {
	h.mu.Lock()
	f := h.feeds[e.CampaignID]
	h.mu.Unlock()
	if f == nil {
		return // no stream for this campaign on this pod
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.ready {
		return
	}
	if e.Seq == 0 {
		for l := range f.members {
			select {
			case l.Live <- e:
			default:
			}
		}
		return
	}
	switch {
	case e.Seq <= f.last:
	case e.Seq == f.last+1:
		h.deliver(f, e)
		h.drainEarly(f)
	default:
		f.early[e.Seq] = e
		h.fill(f)
	}
}

// fill reads what the feed is missing from the log and delivers it. On an error the early events wait
// for the next message or check.
func (h *Hub) fill(f *feed) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for range 20 {
		events, err := h.log.EventsAfter(ctx, f.id, f.last, fillPage)
		if err != nil {
			h.logger.Warn("table gap fill failed", "campaign_id", f.id, "after", f.last, "error", err)
			return
		}
		for _, e := range events {
			if e.Seq == f.last+1 {
				h.deliver(f, e)
			}
		}
		h.drainEarly(f)
		if len(events) < fillPage {
			return
		}
	}
}

func (h *Hub) drainEarly(f *feed) {
	for seq := range f.early {
		if seq <= f.last {
			delete(f.early, seq)
		}
	}
	for {
		e, ok := f.early[f.last+1]
		if !ok {
			return
		}
		delete(f.early, e.Seq)
		h.deliver(f, e)
	}
}

func (h *Hub) deliver(f *feed, e Event) {
	f.last = e.Seq
	for l := range f.members {
		select {
		case l.Events <- e:
		default:
			delete(f.members, l)
			l.drop()
			h.logger.Warn("table stream too slow; dropped (it reconnects and replays)", "campaign_id", f.id, "seq", e.Seq)
		}
	}
}

// Check compares each campaign's last_seq with what this pod delivered, every interval, and fills any
// gap. One query per pod per interval, however many campaigns it serves.
func (h *Hub) Check(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			h.checkOnce(ctx)
		}
	}
}

func (h *Hub) checkOnce(ctx context.Context) {
	h.mu.Lock()
	feeds := make([]*feed, 0, len(h.feeds))
	ids := make([]uuid.UUID, 0, len(h.feeds))
	for id, f := range h.feeds {
		feeds, ids = append(feeds, f), append(ids, id)
	}
	h.mu.Unlock()
	if len(ids) == 0 {
		return
	}
	readCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	seqs, err := h.log.LastSeqs(readCtx, ids)
	if err != nil {
		h.logger.Warn("table check failed", "error", err)
		return
	}
	for _, f := range feeds {
		f.mu.Lock()
		if f.ready && seqs[f.id] > f.last {
			h.logger.Info("table check found missed events", "campaign_id", f.id, "had", f.last, "log", seqs[f.id])
			h.fill(f)
		}
		f.mu.Unlock()
	}
}

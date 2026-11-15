package tabletop

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

// memBus is an in-process bus shared by several hubs (pods). drop decides which messages are lost.
type memBus struct {
	mu   sync.Mutex
	subs map[*memSub]struct{}
	drop func(subject string, data []byte) bool
}

type memSub struct {
	bus     *memBus
	pattern string
	handle  func(string, []byte)
}

func (s *memSub) Unsubscribe() error {
	s.bus.mu.Lock()
	defer s.bus.mu.Unlock()
	delete(s.bus.subs, s)
	return nil
}

func newMemBus() *memBus { return &memBus{subs: map[*memSub]struct{}{}} }

// matches handles the one wildcard the hub uses: `*` as the last token.
func matches(pattern, subject string) bool {
	if prefix, ok := strings.CutSuffix(pattern, ".*"); ok {
		rest, ok := strings.CutPrefix(subject, prefix+".")
		return ok && !strings.Contains(rest, ".")
	}
	return pattern == subject
}

func (b *memBus) Publish(subject string, data []byte) error {
	b.mu.Lock()
	var handlers []func(string, []byte)
	for s := range b.subs {
		if matches(s.pattern, subject) {
			handlers = append(handlers, s.handle)
		}
	}
	drop := b.drop
	b.mu.Unlock()
	for _, h := range handlers {
		if drop == nil || !drop(subject, data) {
			h(subject, data)
		}
	}
	return nil
}

func (b *memBus) Subscribe(subject string, handle func(string, []byte)) (Subscription, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	s := &memSub{bus: b, pattern: subject, handle: handle}
	b.subs[s] = struct{}{}
	return s, nil
}

func (b *memBus) Close() {}

// memLog is the event log, shared by every pod like the database.
type memLog struct {
	mu     sync.Mutex
	events []Event
}

func (l *memLog) commit(campaign uuid.UUID, typ string) Event {
	l.mu.Lock()
	defer l.mu.Unlock()
	e := Event{CampaignID: campaign, Seq: int64(len(l.events)) + 1, Type: typ, Audience: AudienceAll, Payload: []byte(`{}`)}
	l.events = append(l.events, e)
	return e
}

func (l *memLog) EventsAfter(_ context.Context, _ uuid.UUID, after int64, limit int) ([]Event, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []Event
	for _, e := range l.events {
		if e.Seq > after && len(out) < limit {
			out = append(out, e)
		}
	}
	return out, nil
}

func (l *memLog) LastSeqs(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]int64, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := map[uuid.UUID]int64{}
	for _, id := range ids {
		out[id] = int64(len(l.events))
	}
	return out, nil
}

func quietLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// received reads n events or fails after a second.
func received(t *testing.T, l *Listener, n int) []int64 {
	t.Helper()
	var seqs []int64
	for len(seqs) < n {
		select {
		case e := <-l.Events:
			seqs = append(seqs, e.Seq)
		case <-time.After(time.Second):
			t.Fatalf("got %v, want %d events", seqs, n)
		}
	}
	return seqs
}

func nothingMore(t *testing.T, l *Listener) {
	t.Helper()
	select {
	case e := <-l.Events:
		t.Fatalf("unexpected event %d", e.Seq)
	case <-time.After(20 * time.Millisecond):
	}
}

func TestEventsCrossPods(t *testing.T) {
	bus, log := newMemBus(), &memLog{}
	podA, podB := NewHub(bus, log, quietLogger()), NewHub(bus, log, quietLogger())
	ctx := context.Background()
	onA, _ := podA.Join(ctx, campaignID)
	onB, _ := podB.Join(ctx, campaignID)

	// committed through pod A: both pods' streams get it once, A's own echo is dropped
	podA.Publish(ctx, []Event{log.commit(campaignID, TokenMoved)})
	if got := received(t, onA, 1); got[0] != 1 {
		t.Errorf("pod A got %v", got)
	}
	if got := received(t, onB, 1); got[0] != 1 {
		t.Errorf("pod B got %v", got)
	}
	nothingMore(t, onA)
}

func TestLostMessageIsFilledFromTheLog(t *testing.T) {
	bus, log := newMemBus(), &memLog{}
	podA, podB := NewHub(bus, log, quietLogger()), NewHub(bus, log, quietLogger())
	ctx := context.Background()
	onB, _ := podB.Join(ctx, campaignID)

	bus.drop = func(_ string, data []byte) bool { return strings.Contains(string(data), `"seq":1,`) }
	podA.Publish(ctx, []Event{log.commit(campaignID, TokenMoved)}) // lost on the way to pod B
	nothingMore(t, onB)
	bus.drop = nil
	podA.Publish(ctx, []Event{log.commit(campaignID, TokenMoved)})
	if got := received(t, onB, 2); got[0] != 1 || got[1] != 2 {
		t.Errorf("pod B should fill 1 from the log, then 2, got %v", got)
	}
}

func TestOutOfOrderAndDuplicates(t *testing.T) {
	bus, log := newMemBus(), &memLog{}
	pod := NewHub(bus, log, quietLogger())
	ctx := context.Background()
	l, _ := pod.Join(ctx, campaignID)
	first, second, third := log.commit(campaignID, TokenMoved), log.commit(campaignID, TokenMoved), log.commit(campaignID, TokenMoved)

	pod.receive(third) // arrives first: 1 and 2 come from the log
	pod.receive(first)
	pod.receive(second)
	pod.receive(third)
	if got := received(t, l, 3); got[0] != 1 || got[1] != 2 || got[2] != 3 {
		t.Errorf("want 1 2 3 once each, got %v", got)
	}
	nothingMore(t, l)
}

func TestCheckCatchesAPodThatNeverPublished(t *testing.T) {
	bus, log := newMemBus(), &memLog{}
	pod := NewHub(bus, log, quietLogger())
	ctx := context.Background()
	l, _ := pod.Join(ctx, campaignID)
	log.commit(campaignID, TokenMoved) // committed, then the committing pod died
	nothingMore(t, l)
	pod.checkOnce(ctx)
	if got := received(t, l, 1); got[0] != 1 {
		t.Errorf("the check should deliver 1, got %v", got)
	}
}

func TestSlowStreamIsDropped(t *testing.T) {
	bus, log := newMemBus(), &memLog{}
	pod := NewHub(bus, log, quietLogger())
	ctx := context.Background()
	l, _ := pod.Join(ctx, campaignID)
	for range listenerBuffer + 1 {
		pod.receive(log.commit(campaignID, TokenMoved))
	}
	select {
	case <-l.Gone():
	default:
		t.Fatal("a stream that fell a whole buffer behind should be dropped (it replays on reconnect)")
	}
}

func TestLastStreamUnsubscribes(t *testing.T) {
	bus, log := newMemBus(), &memLog{}
	pod := NewHub(bus, log, quietLogger())
	ctx := context.Background()
	a, _ := pod.Join(ctx, campaignID)
	b, _ := pod.Join(ctx, campaignID)
	if len(bus.subs) != 1 {
		t.Fatalf("two streams for one campaign should share one subscription, have %d", len(bus.subs))
	}
	pod.Leave(campaignID, a)
	pod.Leave(campaignID, b)
	if len(bus.subs) != 0 {
		t.Errorf("the last stream leaving should unsubscribe, %d left", len(bus.subs))
	}
	pod.Close()
	if _, err := pod.Join(ctx, campaignID); err != ErrHubClosed {
		t.Errorf("Join after Close: %v", err)
	}
}

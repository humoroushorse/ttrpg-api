package tabletop

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"

	"github.com/humoroushorse/go_dnd/pkg/config"
)

// Bus carries table messages between go_dnd pods: core NATS in production, an in-process fake in
// tests. It delivers at most once and keeps nothing; the hub fills any gap from the log.
type Bus interface {
	Publish(subject string, data []byte) error
	// Subscribe calls handle for each message, one at a time, in the order the bus received them.
	Subscribe(subject string, handle func(subject string, data []byte)) (Subscription, error)
	Close()
}

// Subscription is one Subscribe; Unsubscribe stops it.
type Subscription interface {
	Unsubscribe() error
}

// Subjects (ttrpg-api/docs/nats-subjects.md). A pod subscribes to feedSubject while it has a stream
// for the campaign.
func eventSubject(campaignID uuid.UUID) string {
	return fmt.Sprintf("dnd.campaign.%s.table.event_appended", campaignID)
}
func liveSubject(campaignID uuid.UUID) string {
	return fmt.Sprintf("dnd.campaign.%s.table.live_sent", campaignID)
}
func feedSubject(campaignID uuid.UUID) string {
	return fmt.Sprintf("dnd.campaign.%s.table.*", campaignID)
}

type natsBus struct{ nc *nats.Conn }

// NewNATSBus connects to NATS. It doesn't fail when NATS is down: the pod starts, commands still work,
// tables catch up through the hub's check, and the connection keeps retrying in the background.
func NewNATSBus(cfg config.NATSConfig, logger *slog.Logger) (Bus, error) {
	host, _ := os.Hostname()
	nc, err := nats.Connect(cfg.URL,
		nats.Name("go_dnd "+host),
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(cfg.MaxReconnects),
		nats.ReconnectWait(cfg.ReconnectWait),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			logger.Warn("nats disconnected; tables update every 20 s until it's back", "error", err)
		}),
		nats.ReconnectHandler(func(c *nats.Conn) { logger.Info("nats reconnected", "url", c.ConnectedUrl()) }),
		nats.ErrorHandler(func(_ *nats.Conn, sub *nats.Subscription, err error) {
			subject := ""
			if sub != nil {
				subject = sub.Subject
			}
			logger.Error("nats error", "subject", subject, "error", err)
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("nats connect %s: %w", cfg.URL, err)
	}
	return &natsBus{nc: nc}, nil
}

func (b *natsBus) Publish(subject string, data []byte) error { return b.nc.Publish(subject, data) }

func (b *natsBus) Subscribe(subject string, handle func(string, []byte)) (Subscription, error) {
	return b.nc.Subscribe(subject, func(m *nats.Msg) { handle(m.Subject, m.Data) })
}

// Close drains, so messages already published leave before the connection closes.
func (b *natsBus) Close() { _ = b.nc.Drain() }

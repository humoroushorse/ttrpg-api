# NATS subjects

Every subject any service publishes or subscribes to (`.kiro/steering/nats-messaging-patterns.md`).
Format: `service.entity.{id}.action`, lowercase, past tense, no user ids in the subject.

Every message carries `timestamp`, `trace_id` and `project_id` (the campaign or board id), plus
`user_id` when a user caused it.

## go_dnd

Core NATS only, no JetStream: Postgres holds the durable copy (`DECISIONS.md`, "Many go_dnd pods from
the start"). A pod subscribes to `dnd.campaign.{campaign_id}.table.*` while it has an open table stream
for that campaign, and unsubscribes when the last one closes. Nothing subscribes to all campaigns.

- `dnd.campaign.{campaign_id}.table.event_appended`: a saved table event committed, published by the pod
  that committed it, after the commit.
  - Payload: `{timestamp, trace_id, project_id, user_id, event: {seq, scene_id, type, audience, payload,
    created_at}}`. `event` is the `dnd_vtt.event` row, before any per-viewer filtering; only pods receive
    it.
  - Subscribers deliver events in `seq` order. A gap means they read the missing ones from the log; a
    `seq` they already sent is dropped. Losing a message only delays it.
- `dnd.campaign.{campaign_id}.table.live_sent`: a ping (later a throttled drag preview). Never saved, no
  `seq`; a lost one is just gone.
  - Payload: `{timestamp, trace_id, project_id, user_id, scene_id, kind: "ping", cell: {x, y}}`.

## go_sprint

Subscribed in `go_sprint/pkg/websocket/notifier.go`; their payloads aren't documented here yet.

- `sprint.{sprint_id}.workitem.created`, `.updated`, `.deleted`
- `sprint.{sprint_id}.sprint.created`, `.updated`, `.closed`
- `sprint.{sprint_id}.comment.added`, `.updated`, `.deleted`

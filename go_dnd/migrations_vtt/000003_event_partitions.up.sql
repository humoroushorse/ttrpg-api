-- The event log by month, so history past its retention goes a month at a time: dropping a partition is
-- instant, while deleting that many rows means hours of vacuum. Queries look events up by campaign and
-- seq, not by time, so every query reads every partition's index; months keep that to about four.
--
-- A partitioned table's primary key has to include the partition key, so it becomes
-- (campaign_id, seq, created_at). seq stays unique per campaign because AppendEvent takes it from the
-- locked campaign_state row, not because of the key.
--
-- go_dnd calls create_event_partitions and drop_event_partitions every day (one pod, under an
-- advisory lock). event_default catches rows if that ever falls behind, so writes never fail; it should
-- stay empty, and the job reports it when it isn't.

ALTER TABLE dnd_vtt.event RENAME TO event_unpartitioned;
ALTER INDEX dnd_vtt.pk_event RENAME TO pk_event_unpartitioned;
ALTER INDEX dnd_vtt.ix_dnd_vtt_event_campaign_id_command_id RENAME TO ix_dnd_vtt_event_unpartitioned_command_id;
ALTER INDEX dnd_vtt.ix_dnd_vtt_event_created_by RENAME TO ix_dnd_vtt_event_unpartitioned_created_by;

CREATE TABLE dnd_vtt.event (
    campaign_id uuid NOT NULL,
    seq bigint NOT NULL,
    -- the scene it happened on; null for campaign events. No foreign key: the log outlives a purged scene
    scene_id uuid,
    -- token.moved, token.added, scene.activated, ...
    type character varying NOT NULL,
    -- all or dms. Each recipient's copy is still built per viewer (a token's true name, later HP)
    audience character varying NOT NULL,
    payload jsonb DEFAULT '{}'::jsonb NOT NULL,
    -- the client's id for the command, so a retried request isn't applied twice
    command_id uuid,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    -- who did it
    created_by uuid,
    CONSTRAINT pk_event PRIMARY KEY (campaign_id, seq, created_at),
    CONSTRAINT ck_event_seq CHECK (seq > 0),
    CONSTRAINT ck_event_audience CHECK (audience IN ('all', 'dms')),
    CONSTRAINT fk_event_campaign_id_campaign FOREIGN KEY (campaign_id) REFERENCES dnd.campaign(id) ON DELETE CASCADE,
    CONSTRAINT fk_event_created_by_user FOREIGN KEY (created_by) REFERENCES dnd."user"(id) ON DELETE SET NULL
) PARTITION BY RANGE (created_at);
CREATE INDEX ix_dnd_vtt_event_campaign_id_command_id ON dnd_vtt.event USING btree (campaign_id, command_id) WHERE (command_id IS NOT NULL);
-- the user purge's SET NULL would otherwise scan the whole log, and "what did this player do" uses it
CREATE INDEX ix_dnd_vtt_event_created_by ON dnd_vtt.event USING btree (created_by);

CREATE TABLE dnd_vtt.event_default PARTITION OF dnd_vtt.event DEFAULT;

-- event_YYYY_MM for this month (UTC) through months_ahead months from now. Safe to run any time.
CREATE FUNCTION dnd_vtt.create_event_partitions(months_ahead integer) RETURNS SETOF text
LANGUAGE plpgsql AS $$
DECLARE
    this_month timestamp := date_trunc('month', now() AT TIME ZONE 'UTC');
    month_start timestamp;
    part text;
BEGIN
    FOR i IN 0..months_ahead LOOP
        month_start := this_month + make_interval(months => i);
        part := 'event_' || to_char(month_start, 'YYYY_MM');
        IF to_regclass('dnd_vtt.' || part) IS NULL THEN
            EXECUTE format('CREATE TABLE dnd_vtt.%I PARTITION OF dnd_vtt.event FOR VALUES FROM (%L) TO (%L)',
                           part, month_start AT TIME ZONE 'UTC',
                           (month_start + interval '1 month') AT TIME ZONE 'UTC');
            RETURN NEXT part;
        END IF;
    END LOOP;
END $$;

-- Drops the months whose every row is older than keep, and returns their names.
CREATE FUNCTION dnd_vtt.drop_event_partitions(keep interval) RETURNS SETOF text
LANGUAGE plpgsql AS $$
DECLARE
    part text;
BEGIN
    FOR part IN
        SELECT c.relname::text
        FROM pg_inherits i
        JOIN pg_class c ON c.oid = i.inhrelid
        WHERE i.inhparent = 'dnd_vtt.event'::regclass AND c.relname ~ '^event_[0-9]{4}_[0-9]{2}$'
        ORDER BY c.relname
    LOOP
        IF (to_date(substr(part, 7), 'YYYY_MM') + interval '1 month') AT TIME ZONE 'UTC' <= now() - keep THEN
            EXECUTE format('DROP TABLE dnd_vtt.%I', part);
            RETURN NEXT part;
        END IF;
    END LOOP;
END $$;

SELECT dnd_vtt.create_event_partitions(3);

INSERT INTO dnd_vtt.event (campaign_id, seq, scene_id, type, audience, payload, command_id, created_at, created_by)
SELECT campaign_id, seq, scene_id, type, audience, payload, command_id, created_at, created_by
FROM dnd_vtt.event_unpartitioned;
DROP TABLE dnd_vtt.event_unpartitioned;

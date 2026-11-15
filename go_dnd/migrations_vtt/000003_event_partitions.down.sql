-- Back to one unpartitioned table, as 000001 made it, keeping every row.
ALTER TABLE dnd_vtt.event RENAME TO event_partitioned;
ALTER INDEX dnd_vtt.pk_event RENAME TO pk_event_partitioned;
ALTER INDEX dnd_vtt.ix_dnd_vtt_event_campaign_id_command_id RENAME TO ix_dnd_vtt_event_partitioned_command_id;
ALTER INDEX dnd_vtt.ix_dnd_vtt_event_created_by RENAME TO ix_dnd_vtt_event_partitioned_created_by;

CREATE TABLE dnd_vtt.event (
    campaign_id uuid NOT NULL,
    seq bigint NOT NULL,
    scene_id uuid,
    type character varying NOT NULL,
    audience character varying NOT NULL,
    payload jsonb DEFAULT '{}'::jsonb NOT NULL,
    command_id uuid,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by uuid,
    CONSTRAINT pk_event PRIMARY KEY (campaign_id, seq),
    CONSTRAINT ck_event_seq CHECK (seq > 0),
    CONSTRAINT ck_event_audience CHECK (audience IN ('all', 'dms')),
    CONSTRAINT fk_event_campaign_id_campaign FOREIGN KEY (campaign_id) REFERENCES dnd.campaign(id) ON DELETE CASCADE,
    CONSTRAINT fk_event_created_by_user FOREIGN KEY (created_by) REFERENCES dnd."user"(id) ON DELETE SET NULL
);
CREATE INDEX ix_dnd_vtt_event_campaign_id_command_id ON dnd_vtt.event USING btree (campaign_id, command_id) WHERE (command_id IS NOT NULL);
CREATE INDEX ix_dnd_vtt_event_created_by ON dnd_vtt.event USING btree (created_by);

INSERT INTO dnd_vtt.event SELECT campaign_id, seq, scene_id, type, audience, payload, command_id, created_at, created_by
FROM dnd_vtt.event_partitioned;
DROP TABLE dnd_vtt.event_partitioned;
DROP FUNCTION dnd_vtt.drop_event_partitions(interval);
DROP FUNCTION dnd_vtt.create_event_partitions(integer);

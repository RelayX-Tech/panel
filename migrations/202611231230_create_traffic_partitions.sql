-- +goose Up
-- +goose StatementBegin
CREATE TABLE traffic_samples (
    entity_type varchar(16)  NOT NULL,
    entity_id   varchar(128) NOT NULL,
    ts          timestamptz  NOT NULL,
    up_bytes    bigint       NOT NULL DEFAULT 0,
    down_bytes  bigint       NOT NULL DEFAULT 0,
    connections integer      NOT NULL DEFAULT 0,
    PRIMARY KEY (entity_type, entity_id, ts)
) PARTITION BY RANGE (ts);

CREATE TABLE traffic_1m (LIKE traffic_samples INCLUDING ALL) PARTITION BY RANGE (ts);
CREATE TABLE traffic_1h (LIKE traffic_samples INCLUDING ALL) PARTITION BY RANGE (ts);

-- 当月与下月分区；后续月份由定时任务按同名规则维护。
DO $$
DECLARE
    i          int;
    start_date date;
    end_date   date;
    suffix     text;
BEGIN
    FOR i IN 0..1 LOOP
        start_date := (date_trunc('month', now()) + (i || ' month')::interval)::date;
        end_date := (start_date + interval '1 month')::date;
        suffix := to_char(start_date, 'YYYY"m"MM');
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS %I PARTITION OF traffic_samples FOR VALUES FROM (%L) TO (%L)',
            'traffic_samples_' || suffix, start_date, end_date);
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS %I PARTITION OF traffic_1m FOR VALUES FROM (%L) TO (%L)',
            'traffic_1m_' || suffix, start_date, end_date);
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS %I PARTITION OF traffic_1h FOR VALUES FROM (%L) TO (%L)',
            'traffic_1h_' || suffix, start_date, end_date);
    END LOOP;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE traffic_1h;
DROP TABLE traffic_1m;
DROP TABLE traffic_samples;
-- +goose StatementEnd

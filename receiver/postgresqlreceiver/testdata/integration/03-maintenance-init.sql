CREATE USER otelu WITH PASSWORD 'otelp';
GRANT pg_monitor TO otelu;

-- Autovacuum is disabled on both tables so the only vacuums and analyzes they
-- see are the ones the test runs, which keeps the counts and ages deterministic.
CREATE TABLE maintained (id integer) WITH (autovacuum_enabled = false);
INSERT INTO maintained SELECT g FROM generate_series(1, 100) AS g;

CREATE TABLE untouched (id integer) WITH (autovacuum_enabled = false);

-- A text column gives the table a TOAST table, which is frozen separately from it.
CREATE TABLE toasted (id integer, payload text) WITH (autovacuum_enabled = false, toast.autovacuum_enabled = false);

-- A partitioned table holds no rows itself, so it has no relfrozenxid or relminmxid.
CREATE TABLE parted (id integer) PARTITION BY RANGE (id);
CREATE TABLE parted_1 PARTITION OF parted FOR VALUES FROM (0) TO (1000) WITH (autovacuum_enabled = false);

CREATE USER otelu WITH PASSWORD 'otelp';
GRANT pg_monitor TO otelu;

-- Autovacuum is disabled on both tables so the only vacuums and analyzes they
-- see are the ones the test runs, which keeps the counts and ages deterministic.
CREATE TABLE maintained (id integer) WITH (autovacuum_enabled = false);
INSERT INTO maintained SELECT g FROM generate_series(1, 100) AS g;

CREATE TABLE untouched (id integer) WITH (autovacuum_enabled = false);

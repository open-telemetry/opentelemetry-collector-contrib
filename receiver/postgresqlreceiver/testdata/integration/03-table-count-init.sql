CREATE USER otelu WITH PASSWORD 'otelp';
GRANT SELECT ON pg_stat_database TO otelu;
GRANT pg_monitor TO otelu;

-- Includes a partitioned table and a materialized view, not just plain tables.
CREATE TABLE plain1 (
    id serial PRIMARY KEY
);
CREATE TABLE plain2 (
    id serial PRIMARY KEY
);

CREATE TABLE partitioned (
    id serial,
    created_at date NOT NULL
) PARTITION BY RANGE (created_at);
CREATE TABLE partitioned_2026 PARTITION OF partitioned
    FOR VALUES FROM ('2026-01-01') TO ('2027-01-01');

CREATE MATERIALIZED VIEW plain1_view AS SELECT * FROM plain1;

GRANT SELECT ON plain1, plain2, partitioned, partitioned_2026, plain1_view TO otelu;

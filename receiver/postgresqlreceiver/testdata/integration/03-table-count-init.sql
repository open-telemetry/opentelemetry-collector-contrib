CREATE USER otelu WITH PASSWORD 'otelp';
GRANT pg_monitor TO otelu;

-- Ordinary tables (relkind 'r').
CREATE TABLE ordinary_a (id serial PRIMARY KEY);
CREATE TABLE ordinary_b (id serial PRIMARY KEY);

-- Partitioned table: the parent is relkind 'p' and each partition is relkind 'r'.
-- All of them appear as rows in pg_stat_user_tables.
CREATE TABLE partitioned (id int) PARTITION BY RANGE (id);
CREATE TABLE partitioned_p1 PARTITION OF partitioned FOR VALUES FROM (0) TO (100);
CREATE TABLE partitioned_p2 PARTITION OF partitioned FOR VALUES FROM (100) TO (200);

-- Materialized view (relkind 'm'), which also appears in pg_stat_user_tables.
CREATE MATERIALIZED VIEW mat_view AS SELECT id FROM ordinary_a;

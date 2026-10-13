CREATE USER otelu WITH PASSWORD 'otelp';
GRANT SELECT ON pg_stat_database TO otelu;
GRANT pg_monitor TO otelu;

-- big_index_table: enough rows for its primary-key btree index to be a
-- meaningful, multi-page size on its own (a handful of rows would round to
-- the same single page as an empty index, making the assertion flaky).
-- Measured on postgres:17.2: 5000 rows -> index size roughly matches the
-- data size, so pg_total_relation_size is comfortably ~2x pg_relation_size.
CREATE TABLE big_index_table (
    id serial PRIMARY KEY,
    val integer NOT NULL
);
INSERT INTO big_index_table (val)
SELECT g FROM generate_series(1, 5000) AS g;

-- toasted_table: a wide text column pushed out-of-line into TOAST storage.
-- Postgres only TOASTs values that make a row exceed roughly a quarter of
-- the page size, so each value here is well past that threshold. Measured
-- on postgres:17.2: 50 such rows already push the TOAST relation to several
-- times the size of the main heap and its index combined.
CREATE TABLE toasted_table (
    id serial PRIMARY KEY,
    payload text NOT NULL
);
INSERT INTO toasted_table (payload)
SELECT repeat('x', 20000) FROM generate_series(1, 50);

VACUUM ANALYZE big_index_table;
VACUUM ANALYZE toasted_table;

GRANT SELECT ON big_index_table, toasted_table TO otelu;

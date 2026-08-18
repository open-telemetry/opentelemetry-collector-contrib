-- Seed data for the pgvector extension-gate integration test. This database has
-- pg_trgm (which overloads the <-> operator) but NOT pgvector, so the collector
-- must emit zero vector-search metrics even though pg_trgm distance operators are
-- present in pg_stat_statements.
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;

CREATE TABLE docs (id serial PRIMARY KEY, body text);
INSERT INTO docs (body) SELECT 'sample ' || g FROM generate_series(1, 20) g;

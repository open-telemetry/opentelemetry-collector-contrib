-- Seed data for the pgvector similarity-search classification integration test.
-- Installs pgvector alongside pg_trgm so the test also exercises the operator
-- collision guards (pg_trgm word-similarity operators must NOT be classified as
-- vector searches).
CREATE EXTENSION IF NOT EXISTS vector;
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;

-- Vector table: exercises cosine/l2/inner_product/l1 operators and functions.
CREATE TABLE items (id serial PRIMARY KEY, embedding vector(3));
INSERT INTO items (embedding) VALUES
    ('[0.1,0.2,0.3]'),
    ('[0.2,0.3,0.4]'),
    ('[0.9,0.8,0.7]'),
    ('[0.4,0.4,0.4]'),
    ('[0.5,0.1,0.9]');

-- Bit table: exercises hamming/jaccard operators and functions.
CREATE TABLE bitems (id serial PRIMARY KEY, b bit(8));
INSERT INTO bitems (b) VALUES
    (B'10101010'),
    (B'11110000'),
    (B'00001111'),
    (B'10011001'),
    (B'01100110');

-- Trap: column names that contain distance-function substrings. Word-boundary
-- anchored regexes must NOT classify queries over these columns as vector searches.
CREATE TABLE fp_cols (
    id int,
    cosine_distance_threshold double precision,
    inner_product_score double precision,
    my_l2_distance_col double precision
);
INSERT INTO fp_cols SELECT g, 0, 0, 0 FROM generate_series(1, 5) g;

-- Trap: text table for pg_trgm word-similarity operators (<<->, <->>) which must
-- NOT be classified as l2 vector searches.
CREATE TABLE docs (id serial PRIMARY KEY, body text);
INSERT INTO docs (body) SELECT 'sample ' || g FROM generate_series(1, 20) g;

-- Trap: table whose name is a prefix of the vector table "items" but has no vector
-- column. Inserts here must NOT be counted as vector inserts.
CREATE TABLE items_archive (id serial PRIMARY KEY, note text);

CREATE USER otelu WITH PASSWORD 'otelp';
GRANT SELECT ON pg_stat_database TO otelu;
GRANT pg_monitor TO otelu;


CREATE TABLE table1 (
    id serial PRIMARY KEY
);
CREATE TABLE table2 (
    id serial PRIMARY KEY
);

CREATE DATABASE otel2;
\c otel2
CREATE TABLE test1 (
    id serial PRIMARY KEY
);
CREATE TABLE test2 (
    id serial PRIMARY KEY
);

CREATE INDEX otelindex ON test1(id);
CREATE INDEX otel2index ON test2(id);

-- Generating usage of index
INSERT INTO test2 (id)
VALUES(67);
SELECT * FROM test2; 


CREATE EXTENSION pg_stat_statements;

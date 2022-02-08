CREATE USER otel WITH PASSWORD 'otel';
GRANT SELECT ON pg_stat_database TO otel;

CREATE TABLE table1 ();
CREATE TABLE table2 ();

CREATE DATABASE otel2;
\c otel2
CREATE TABLE test1 ();
CREATE TABLE test2 ();

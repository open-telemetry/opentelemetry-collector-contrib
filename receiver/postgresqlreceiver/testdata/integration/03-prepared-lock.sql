\c otel2
-- Hold a relation lock from a prepared transaction in a non-default database.
-- Prepared transactions survive server restart and have a NULL pid in
-- pg_locks, so this deterministically covers both cross-database lock
-- collection and counting of locks that are not held by a live backend.
-- Requires max_prepared_transactions > 0.
BEGIN;
LOCK TABLE test1 IN ROW EXCLUSIVE MODE;
PREPARE TRANSACTION 'otel_integration_locks';

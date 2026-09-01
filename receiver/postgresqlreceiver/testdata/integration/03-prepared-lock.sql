\c otel2
-- Hold a relation lock in a non-default database via a prepared transaction:
-- it survives server restart and has a NULL pid in pg_locks, covering both
-- cross-database lock collection and counting of locks with no live backend.
-- Requires max_prepared_transactions > 0.
BEGIN;
LOCK TABLE test1 IN ROW EXCLUSIVE MODE;
PREPARE TRANSACTION 'otel_integration_locks';

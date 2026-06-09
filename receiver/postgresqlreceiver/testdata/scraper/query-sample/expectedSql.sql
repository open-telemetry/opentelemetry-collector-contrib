SELECT
    COALESCE(sa.datname, '') AS datname,
    COALESCE(sa.usename, '') AS usename,
    COALESCE(sa.client_addr::TEXT, '') AS client_addr,
    COALESCE(sa.client_hostname, '') AS client_hostname,
    COALESCE(sa.client_port::TEXT, '') AS client_port,
    COALESCE(sa.query_start::TEXT, '') AS query_start,
    COALESCE(sa.wait_event_type, '') AS wait_event_type,
    COALESCE(sa.wait_event, '') AS wait_event,
    COALESCE(sa.query_id::TEXT, '') AS query_id,
    COALESCE(sa.pid::TEXT, '') AS pid,
    COALESCE(sa.application_name::TEXT, '') AS application_name,
    EXTRACT(EPOCH FROM sa.query_start) AS _query_start_timestamp,
    sa.state,
    sa.query,
    CASE
    WHEN sa.state = 'active' THEN
      EXTRACT(EPOCH FROM (clock_timestamp() - sa.query_start)) * 1e3
    WHEN sa.state IN ('idle','idle in transaction','idle in transaction (aborted)')
         AND sa.state_change IS NOT NULL THEN
      EXTRACT(EPOCH FROM (sa.state_change - sa.query_start)) * 1e3
    ELSE
      NULL
    END AS duration_ms,
    -- Blocking info
    COALESCE(pg_blocking_pids(sa.pid)::TEXT, '{}') AS blocking_pids,
    CASE WHEN bl.waitstart IS NOT NULL
         THEN to_char(bl.waitstart AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
         ELSE '' END AS blocking_start_time,
    COALESCE(EXTRACT(EPOCH FROM (clock_timestamp() - bl.waitstart))::BIGINT, 0) AS blocking_wait_duration,
    COALESCE(bl.mode, '') AS blocking_lock_mode,
    COALESCE(bl.locktype, '') AS blocking_lock_type,
    COALESCE(c.relname, '') AS blocking_lock_relation,
    CASE WHEN sa.xact_start IS NOT NULL
         THEN to_char(sa.xact_start AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
         ELSE '' END AS blocking_transaction_start
FROM pg_stat_activity sa
LEFT JOIN LATERAL (
  SELECT mode, locktype, relation, sa.state_change AS waitstart
  FROM pg_locks
  WHERE pid = sa.pid AND NOT granted
  LIMIT 1
) bl ON TRUE
LEFT JOIN pg_class c
  ON  c.oid = bl.relation
WHERE
    coalesce(
      TRIM(sa.query),
      ''
    ) != ''

    AND sa.pid != pg_backend_pid()

    AND sa.query_start IS NOT NULL
    AND NOT (

      sa.query_start < TO_TIMESTAMP(123440.111)
      AND sa.state = 'idle'
    )
    AND (
      -- Active sessions
      sa.state = 'active'
      -- Idle sessions that are blocking others (idle in transaction holding locks)
      OR sa.pid IN (
        SELECT unnest(pg_blocking_pids(pid))
        FROM   pg_stat_activity
        WHERE  cardinality(pg_blocking_pids(pid)) > 0
      )
    )
LIMIT 30;

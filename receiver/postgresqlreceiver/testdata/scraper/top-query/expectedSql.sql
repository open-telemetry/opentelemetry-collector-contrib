SELECT
  SUM(calls) AS calls,
  datname,
  SUM(shared_blks_dirtied) AS shared_blks_dirtied,
  SUM(shared_blks_hit) AS shared_blks_hit,
  SUM(shared_blks_read) AS shared_blks_read,
  SUM(shared_blks_written) AS shared_blks_written,
  SUM(temp_blks_read) AS temp_blks_read,
  SUM(temp_blks_written) AS temp_blks_written,
  MAX(query) AS query,
  queryid::TEXT,
  userid::TEXT,
  COALESCE(rolname, '') AS rolname,
  SUM(rows)::TEXT AS rows,
  SUM(total_exec_time) AS total_exec_time,
  SUM(total_plan_time) AS total_plan_time
FROM
  pg_stat_statements as pg_stat_statements
  LEFT JOIN pg_roles ON pg_stat_statements.userid = pg_roles.oid
  INNER JOIN pg_database ON pg_stat_statements.dbid = pg_database.oid
WHERE
  datname IS NOT NULL
  AND query != '<insufficient privilege>'
  AND query NOT LIKE '/* otel-collector-ignore */%'
GROUP BY pg_stat_statements.dbid, datname, userid, queryid, rolname
ORDER BY calls DESC
LIMIT 31;
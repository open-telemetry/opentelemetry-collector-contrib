SELECT
    CONVERT(NVARCHAR, TODATETIMEOFFSET(CURRENT_TIMESTAMP, DATEPART(TZOFFSET, SYSDATETIMEOFFSET())), 126) AS now,
    CONVERT(NVARCHAR, TODATETIMEOFFSET(req.start_time, DATEPART(TZOFFSET, SYSDATETIMEOFFSET())), 126) AS query_start,
    sess.login_name AS user_name,
    sess.last_request_start_time AS last_request_start_time,
    sess.session_id AS id,
    DB_NAME(sess.database_id) AS database_name,
    sess.status AS session_status,
    req.status AS request_status,
    SUBSTRING(qt.text, (req.statement_start_offset / 2) + 1,
              ((CASE req.statement_end_offset
                    WHEN -1 THEN DATALENGTH(qt.text)
                    ELSE req.statement_end_offset
                    END - req.statement_start_offset) / 2) + 1) AS statement_text,
    SUBSTRING(qt.text, 1, 500) AS text,
    c.client_tcp_port AS client_port,
    c.client_net_address AS client_address,
    sess.host_name AS host_name,
    sess.program_name AS program_name,
    sess.is_user_process AS is_user_process,
    req.command,
    req.blocking_session_id,
    req.wait_type,
    req.wait_time,
    req.last_wait_type,
    req.wait_resource,
    req.open_transaction_count,
    req.transaction_id,
    req.percent_complete,
    req.estimated_completion_time,
    req.cpu_time,
    req.total_elapsed_time,
    req.reads,
    req.writes,
    req.logical_reads,
    req.transaction_isolation_level,
    req.lock_timeout,
    req.deadlock_priority,
    req.row_count,
    req.query_hash,
    req.query_plan_hash,
    req.context_info
FROM
    sys.dm_exec_sessions sess
        INNER JOIN
    sys.dm_exec_connections c ON sess.session_id = c.session_id
        INNER JOIN
    sys.dm_exec_requests req ON c.connection_id = req.connection_id
        CROSS APPLY
    sys.dm_exec_sql_text(req.sql_handle) qt
WHERE
    sess.status != 'sleeping';

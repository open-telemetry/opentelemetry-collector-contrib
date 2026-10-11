CREATE MATERIALIZED VIEW %s_summary AS
SELECT
    trace_id AS s_trace_id,
    date_trunc(timestamp, 'day') AS s_day,
    MIN(timestamp) AS s_start_time,
    MAX(end_time) AS s_end_time,
    COUNT(*) AS s_span_count,
    SUM(CASE WHEN status_code = 'STATUS_CODE_ERROR' THEN 1 ELSE 0 END) AS s_error_count,
    MIN(CASE WHEN is_root = 1 THEN CONCAT(DATE_FORMAT(timestamp, '%%Y%%m%%d%%H%%i%%s%%f'), '|', service_name, '|', span_name) END) AS s_first_root,
    MAX(duration) AS s_max_span_duration
FROM %s
GROUP BY trace_id, date_trunc(timestamp, 'day');

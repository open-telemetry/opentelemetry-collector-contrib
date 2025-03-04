CREATE JOB `%s:%s_graph_job`
ON SCHEDULE EVERY 10 MINUTE
DO
INSERT INTO %s_graph
SELECT
    date_trunc(t2.timestamp, 'MINUTE') as timestamp,
    t1.service_name AS caller_service_name,
    t1.service_instance_id AS caller_service_instance_id,
    t2.service_name AS callee_service_name,
    t2.service_instance_id AS callee_service_instance_id,
    count(*) as count,
    sum(if(t2.status_code = 'STATUS_CODE_ERROR', 1, 0)) as error_count
FROM %s t1
JOIN %s t2
ON t1.trace_id = t2.trace_id
AND t1.span_id != ''
AND t1.service_name != t2.service_name
AND t1.span_id = t2.parent_span_id
AND t2.timestamp >= minutes_sub(date_trunc(now(), 'MINUTE'), 10)
GROUP BY timestamp, caller_service_name, caller_service_instance_id, callee_service_name, callee_service_instance_id;

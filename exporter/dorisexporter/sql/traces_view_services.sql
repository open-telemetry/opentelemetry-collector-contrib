CREATE MATERIALIZED VIEW %s_services AS
SELECT service_name AS svc, service_instance_id AS inst, span_name AS span
FROM %s
GROUP BY service_name, service_instance_id, span_name;

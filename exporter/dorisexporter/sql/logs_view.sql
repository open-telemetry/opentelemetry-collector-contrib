CREATE MATERIALIZED VIEW %s_services AS
SELECT service_name AS svc, service_instance_id AS inst
FROM %s
GROUP BY service_name, service_instance_id;

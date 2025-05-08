CREATE MATERIALIZED VIEW %s_services AS 
SELECT service_name, service_instance_id, span_name 
FROM %s
GROUP BY service_name, service_instance_id, span_name;

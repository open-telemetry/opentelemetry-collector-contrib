CREATE MATERIALIZED VIEW %s_services AS 
SELECT service_name, service_instance_id 
FROM %s
GROUP BY service_name, service_instance_id;

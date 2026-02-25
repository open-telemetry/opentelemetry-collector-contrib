CREATE TABLE IF NOT EXISTS %s_graph
(
    timestamp                   DATETIME(6),
    caller_service_name         VARCHAR(200),
    caller_service_instance_id  VARCHAR(200),
    callee_service_name         VARCHAR(200),
    callee_service_instance_id  VARCHAR(200),
    count                       BIGINT,
    error_count                 BIGINT,
    INDEX idx_timestamp(timestamp) USING INVERTED,
    INDEX idx_caller_service_name(caller_service_name) USING INVERTED,
    INDEX idx_caller_service_instance_id(caller_service_instance_id) USING INVERTED,
    INDEX idx_callee_service_name(callee_service_name) USING INVERTED,
    INDEX idx_callee_service_instance_id(callee_service_instance_id) USING INVERTED,
    INDEX count(count) USING INVERTED,
    INDEX error_count(error_count) USING INVERTED
)
UNIQUE KEY(timestamp, caller_service_name, caller_service_instance_id, callee_service_name, callee_service_instance_id)
PARTITION BY RANGE(timestamp) ()
DISTRIBUTED BY HASH(caller_service_name) BUCKETS AUTO
%s;

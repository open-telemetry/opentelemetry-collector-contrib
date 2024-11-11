CREATE TABLE IF NOT EXISTS %s_summary
(
    service_name          VARCHAR(200),
    timestamp             DATETIME(6),
    metric_name           VARCHAR(200),
    metric_description    STRING,
    metric_unit           STRING,
    attributes            VARIANT,
    start_time            DATETIME(6),
    count                 BIGINT,
    sum                   DOUBLE,
    quantile_values       ARRAY<STRUCT<quantile:DOUBLE, value:DOUBLE>>,
    resource_attributes   VARIANT,
    scope_name            STRING,
    scope_version         STRING,
    INDEX idx_service_name(service_name) USING INVERTED,
    INDEX idx_timestamp(timestamp) USING INVERTED,
    INDEX idx_metric_name(metric_name) USING INVERTED,
    INDEX idx_metric_description(metric_description) USING INVERTED,
    INDEX idx_metric_unit(metric_unit) USING INVERTED,
    INDEX idx_attributes(attributes) USING INVERTED,
    INDEX idx_start_time(start_time) USING INVERTED,
    INDEX idx_count(count) USING INVERTED,
    INDEX idx_resource_attributes(resource_attributes) USING INVERTED,
    INDEX idx_scope_name(scope_name) USING INVERTED,
    INDEX idx_scope_version(scope_version) USING INVERTED
)
ENGINE = OLAP
DUPLICATE KEY(service_name, timestamp)
PARTITION BY RANGE(timestamp) ()
DISTRIBUTED BY HASH(metric_name) BUCKETS AUTO
%s;
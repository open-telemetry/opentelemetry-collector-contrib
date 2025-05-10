CREATE TABLE IF NOT EXISTS %s_exponential_histogram
(
    service_name            VARCHAR(200),
    timestamp               DATETIME(6),
    metric_name             VARCHAR(200),
    metric_description      STRING,
    metric_unit             STRING,
    attributes              VARIANT,
    start_time              DATETIME(6),
    count                   BIGINT,
    sum                     DOUBLE,
    scale                   INT,
    zero_count              BIGINT,
    positive_offset         INT,
    positive_bucket_counts  ARRAY<BIGINT>,
    negative_offset         INT,
    negative_bucket_counts  ARRAY<BIGINT>,
    exemplars               ARRAY<STRUCT<filtered_attributes:MAP<STRING,STRING>, timestamp:DATETIME(6), value:DOUBLE, span_id:STRING, trace_id:STRING>>,
    min                     DOUBLE,
    max                     DOUBLE,
    zero_threshold          DOUBLE,
    aggregation_temporality STRING,
    resource_attributes     VARIANT,
    scope_name              STRING,
    scope_version           STRING,
    INDEX idx_service_name(service_name) USING INVERTED,
    INDEX idx_timestamp(timestamp) USING INVERTED,
    INDEX idx_metric_name(metric_name) USING INVERTED,
    INDEX idx_metric_description(metric_description) USING INVERTED,
    INDEX idx_metric_unit(metric_unit) USING INVERTED,
    INDEX idx_attributes(attributes) USING INVERTED,
    INDEX idx_start_time(start_time) USING INVERTED,
    INDEX idx_count(count) USING INVERTED,
    INDEX idx_scale(scale) USING INVERTED,
    INDEX idx_zero_count(zero_count) USING INVERTED,
    INDEX idx_positive_offset(positive_offset) USING INVERTED,
    INDEX idx_negative_offset(negative_offset) USING INVERTED,
    INDEX idx_aggregation_temporality(aggregation_temporality) USING INVERTED,
    INDEX idx_resource_attributes(resource_attributes) USING INVERTED,
    INDEX idx_scope_name(scope_name) USING INVERTED,
    INDEX idx_scope_version(scope_version) USING INVERTED
)
ENGINE = OLAP
DUPLICATE KEY(service_name, timestamp)
PARTITION BY RANGE(timestamp) ()
DISTRIBUTED BY HASH(metric_name) BUCKETS AUTO
%s;

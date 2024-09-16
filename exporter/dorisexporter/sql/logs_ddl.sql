CREATE TABLE IF NOT EXISTS %s
(
    service_name          VARCHAR(200),
    timestamp             DATETIME(6),
    trace_id              VARCHAR(200),
    span_id               STRING,
    severity_number       INT,
    severity_text         STRING,
    body                  STRING,
    resource_attributes   VARIANT,
    log_attributes        VARIANT,
    scope_name            STRING,
    scope_version         STRING,
    INDEX idx_service_name(service_name) USING INVERTED,
    INDEX idx_timestamp(timestamp) USING INVERTED,
    INDEX idx_trace_id(trace_id) USING INVERTED,
    INDEX idx_span_id(span_id) USING INVERTED,
    INDEX idx_severity_number(severity_number) USING INVERTED,
    INDEX idx_body(body) USING INVERTED PROPERTIES("parser"="unicode", "support_phrase"="true"),
    INDEX idx_severity_text(severity_text) USING INVERTED,
    INDEX idx_resource_attributes(resource_attributes) USING INVERTED,
    INDEX idx_log_attributes(log_attributes) USING INVERTED,
    INDEX idx_scope_name(scope_name) USING INVERTED,
    INDEX idx_scope_version(scope_version) USING INVERTED
)
ENGINE = OLAP
DUPLICATE KEY(service_name, timestamp)
PARTITION BY RANGE(timestamp) ()
DISTRIBUTED BY HASH(trace_id) BUCKETS AUTO
%s;
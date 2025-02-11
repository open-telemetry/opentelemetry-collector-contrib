CREATE TABLE IF NOT EXISTS %s	
(	
    service_name          VARCHAR(200),	
    timestamp             DATETIME(6),	
    trace_id              VARCHAR(200),	
    span_id               STRING,	
    trace_state           STRING,	
    parent_span_id        STRING,	
    span_name             STRING,	
    span_kind             STRING,	
    end_time              DATETIME(6),	
    duration              BIGINT,	
    span_attributes       VARIANT,	
    events                ARRAY<STRUCT<timestamp:DATETIME(6), name:STRING, attributes:MAP<STRING, STRING>>>,	
    links                 ARRAY<STRUCT<trace_id:STRING, span_id:STRING, trace_state:STRING, attributes:MAP<STRING, STRING>>>,	
    status_message        STRING,	
    status_code           STRING,	
    resource_attributes   VARIANT,	
    scope_name            STRING,	
    scope_version         STRING,	
    INDEX idx_service_name(service_name) USING INVERTED,	
    INDEX idx_timestamp(timestamp) USING INVERTED,	
    INDEX idx_trace_id(trace_id) USING INVERTED,	
    INDEX idx_span_id(span_id) USING INVERTED,	
    INDEX idx_trace_state(trace_state) USING INVERTED,	
    INDEX idx_parent_span_id(parent_span_id) USING INVERTED,	
    INDEX idx_span_name(span_name) USING INVERTED,	
    INDEX idx_span_kind(span_kind) USING INVERTED,	
    INDEX idx_end_time(end_time) USING INVERTED,	
    INDEX idx_duration(duration) USING INVERTED,	
    INDEX idx_span_attributes(span_attributes) USING INVERTED,	
    INDEX idx_status_message(status_message) USING INVERTED,	
    INDEX idx_status_code(status_code) USING INVERTED,	
    INDEX idx_resource_attributes(resource_attributes) USING INVERTED,	
    INDEX idx_scope_name(scope_name) USING INVERTED,	
    INDEX idx_scope_version(scope_version) USING INVERTED	
)	
ENGINE = OLAP	
DUPLICATE KEY(service_name, timestamp)	
PARTITION BY RANGE(timestamp) ()	
DISTRIBUTED BY HASH(trace_id) BUCKETS AUTO	
%s;

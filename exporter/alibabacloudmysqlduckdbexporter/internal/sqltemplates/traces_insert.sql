INSERT INTO `%s`.`%s` (
    timestamp,
    trace_id,
    span_id,
    parent_span_id,
    trace_state,
    span_name,
    span_kind,
    service_name,
    resource_attributes,
    scope_name,
    scope_version,
    span_attributes,
    duration,
    status_code,
    status_message,
    events,
    links
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)

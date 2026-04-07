INSERT INTO `%s`.`%s` (
    timestamp,
    trace_id,
    span_id,
    trace_flags,
    severity_text,
    severity_number,
    service_name,
    body,
    resource_schema_url,
    resource_attributes,
    scope_schema_url,
    scope_name,
    scope_version,
    scope_attributes,
    log_attributes
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)

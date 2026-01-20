INSERT INTO `%s`.`%s` (
    `ServiceName`,
    `Timestamp`,
    `TraceId`,
    `SpanId`,
    `TraceFlags`,
    `SeverityText`,
    `SeverityNumber`,
    `Body`,
    `ResourceSchemaUrl`,
    `ResourceAttributes`,
    `ScopeSchemaUrl`,
    `ScopeName`,
    `ScopeVersion`,
    `ScopeAttributes`,
    `LogAttributes`
    %s
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ? %s)




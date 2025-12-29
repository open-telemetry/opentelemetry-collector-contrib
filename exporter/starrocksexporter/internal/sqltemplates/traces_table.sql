CREATE TABLE IF NOT EXISTS `%s`.`%s` (
    `ServiceName` VARCHAR(255),
    `SpanName` VARCHAR(255),
    `Timestamp` DATETIME NOT NULL,
    `TraceId` VARCHAR(32) NOT NULL,
    `SpanId` VARCHAR(16) NOT NULL,
    `ParentSpanId` VARCHAR(16),
    `TraceState` VARCHAR(255),
    `SpanKind` VARCHAR(50),
    `ResourceAttributes` JSON,
    `ScopeName` VARCHAR(255),
    `ScopeVersion` VARCHAR(50),
    `SpanAttributes` JSON,
    `Duration` BIGINT,
    `StatusCode` VARCHAR(50),
    `StatusMessage` TEXT,
    `Events` JSON,
    `Links` JSON
) ENGINE=OLAP
DUPLICATE KEY(`ServiceName`, `SpanName`, `Timestamp`, `TraceId`, `SpanId`)
PARTITION BY RANGE(`Timestamp`) (
    PARTITION p20251229 VALUES [("2025-12-29 00:00:00"), ("2025-12-30 00:00:00"))
)
DISTRIBUTED BY HASH(`ServiceName`, `TraceId`) BUCKETS 10
PROPERTIES (
    "replication_num" = "1",
    "dynamic_partition.enable" = "true",
    "dynamic_partition.time_unit" = "DAY",
    "dynamic_partition.start" = "-90",
    "dynamic_partition.end" = "3",
    "dynamic_partition.prefix" = "p",
    "dynamic_partition.buckets" = "10"
)




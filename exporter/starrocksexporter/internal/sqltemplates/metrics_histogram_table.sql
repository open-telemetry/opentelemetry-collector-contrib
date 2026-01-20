CREATE TABLE IF NOT EXISTS `%s`.`%s` (
    `ServiceName` VARCHAR(255),
    `MetricName` VARCHAR(255),
    `TimeUnix` DATETIME NOT NULL,
    `ResourceAttributes` JSON,
    `ResourceSchemaUrl` VARCHAR(500),
    `ScopeName` VARCHAR(255),
    `ScopeVersion` VARCHAR(50),
    `ScopeAttributes` JSON,
    `ScopeDroppedAttrCount` INT,
    `ScopeSchemaUrl` VARCHAR(500),
    `MetricDescription` VARCHAR(500),
    `MetricUnit` VARCHAR(50),
    `Attributes` JSON,
    `StartTimeUnix` DATETIME,
    `Count` BIGINT,
    `Sum` DOUBLE,
    `BucketCounts` JSON,
    `ExplicitBounds` JSON,
    `Exemplars` JSON,
    `Flags` INT,
    `Min` DOUBLE,
    `Max` DOUBLE,
    `AggregationTemporality` INT
) ENGINE=OLAP
DUPLICATE KEY(`ServiceName`, `MetricName`, `TimeUnix`)
PARTITION BY RANGE(`TimeUnix`) (
    PARTITION p20251229 VALUES [("2025-12-29 00:00:00"), ("2025-12-30 00:00:00"))
)
DISTRIBUTED BY HASH(`ServiceName`, `MetricName`) BUCKETS 10
PROPERTIES (
    "replication_num" = "1",
    "dynamic_partition.enable" = "true",
    "dynamic_partition.time_unit" = "DAY",
    "dynamic_partition.start" = "-90",
    "dynamic_partition.end" = "3",
    "dynamic_partition.prefix" = "p",
    "dynamic_partition.buckets" = "10"
)




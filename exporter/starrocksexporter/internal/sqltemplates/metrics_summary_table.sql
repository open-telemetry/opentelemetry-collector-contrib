CREATE TABLE IF NOT EXISTS `%s`.`%s` (
    `ServiceName` VARCHAR(255),
    `MetricName` VARCHAR(255),
    `TimeUnix` DATETIME NOT NULL,
    `ResourceAttributes` JSON,
    `ResourceSchemaUrl` VARCHAR(500),
    `ScopeName` VARCHAR(255),
    `ScopeVersion` VARCHAR(50),
    `ScopeAttributes` JSON,
    `ScopeDroppedAttrCount` INT UNSIGNED,
    `ScopeSchemaUrl` VARCHAR(500),
    `MetricDescription` VARCHAR(500),
    `MetricUnit` VARCHAR(50),
    `Attributes` JSON,
    `StartTimeUnix` DATETIME,
    `Count` BIGINT UNSIGNED,
    `Sum` DOUBLE,
    `ValueAtQuantiles` JSON,
    `Flags` INT UNSIGNED
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




# Version Compatibility

This document lists every version-gated capability in the MySQL receiver. Version detection runs once at `Connect()` time via `fetchDBVersion()`. If detection fails, all predicates return `false`, which falls back to MySQL <8 behavior.

## Capability Predicates

| Predicate | Minimum Version | Fallback Behavior |
|---|---|---|
| `supportsQuerySampleText()` | MySQL 8.0.3+ | Top-query scraper uses 5-column fallback template (`topQueryNoSampleText.tmpl`); `querySampleText` is empty and `EXPLAIN` is skipped |
| `supportsReplicaStatus()` | MySQL 8.0.22+ | `SHOW SLAVE STATUS` is used instead of `SHOW REPLICA STATUS` |
| `supportsProcesslist()` | MySQL 8.0.22+ | `client.port` and `network.peer.port` remain `0`; `information_schema.PROCESSLIST` is **not** used as a fallback (it holds a global mutex, was deprecated in MySQL 8.0, removed in MySQL 9.0, and has already been removed from this receiver) |

## Timer Wait Tiers (`querySample.tmpl`)

Query sample duration is resolved in order. The first tier that produces a value is used.

| Tier | Source | Availability |
|---|---|---|
| 1 | Exact `TIMER_WAIT` for completed waits | All supported versions |
| 2 | PS timer approximation for in-progress waits | MySQL 5.7+ / 8.0+; **not used for MariaDB** — MariaDB's `statement.TIMER_WAIT` is updated only at yield points, not continuously, making it unreliable for in-progress statements |
| 3 | `thread.processlist_time` integer-second fallback | MySQL 5.7+, all supported MariaDB versions |

## Version Detection

- Detection runs **once** at `Connect()` time via `fetchDBVersion()`.
- Failure is **non-fatal**: `dbVersion` stays at its zero value and `Connect()` returns `nil`. All capability predicates return `false`, which produces MySQL <8 fallback behavior.
- The `dbVersion` struct holds two fields: `product` (MySQL or MariaDB) and `version` (semver).
- Connection errors surface on the first scrape, not at connect time.

## Tested Platforms

The following product/version/platform combinations have been validated against a live deployment. Each row reflects the capability matrix observed for that exact configuration.

- **Exact Version** — the full string returned by `SELECT VERSION()` on the tested instance
- **Platform** — deployment type and instance class (e.g. `AWS RDS db.t3.micro`, `Docker 27.x`, `bare metal`)
- **Date** — date live validation passed

| Product | Series | Exact Version | Platform | `supportsQuerySampleText` | `supportsProcesslist` | `supportsReplicaStatus` | Timer Wait Tiers | Date |
|---|---|---|---|---|---|---|---|---|
| MySQL | 8.4 | 8.4.7 | AWS RDS db.t3.micro | ✓ | ✓ | ✓ | 1, 2, 3 | 2026-04-21 |
| MySQL | 5.7 | 5.7.44 | AWS RDS db.t3.micro | ✗ | ✗ | ✗ | 1, 2, 3 | 2026-04-21 |
| MariaDB | 10.5 | 10.5.28 | AWS RDS db.t3.micro | ✗ | ✗ | ✗ | 1, 3 | 2026-04-21 |
| MariaDB | 11.8 | 11.8.2 | AWS RDS db.t3.micro | ✗ | ✗ | ✗ | 1, 3 | 2026-04-21 |

**Legend:**
- ✓ = capability enabled
- ✗ = capability disabled (fallback behavior active)

## `events_waits_current` Consumer

The `mysql.events_waits_current.timer_wait` attribute requires the `events_waits_current`
Performance Schema consumer. This consumer is **disabled by default** on all supported platforms,
including AWS RDS.

### Enablement by platform

| Platform | Method | Persistent? |
|---|---|---|
| MySQL / MariaDB — manual server | `performance-schema-consumer-events-waits-current=ON` in `[mysqld]` (`my.cnf`) | Yes |
| AWS RDS — MySQL / MariaDB | `UPDATE performance_schema.setup_consumers SET ENABLED='YES' WHERE NAME='events_waits_current'` | No — resets on restart/failover |

AWS RDS does not expose this as a parameter group setting. The runtime `UPDATE` must be re-applied
after each restart. The receiver user needs `UPDATE ON performance_schema.setup_consumers` for this.

### Expected behavior once enabled

| Product | Timer Wait Tier Used | Notes |
|---|---|---|
| MySQL 5.7 | Tier 1 or 2 | Tier 1 if wait has completed; Tier 2 (PS timer approximation) if wait is in progress |
| MySQL 8.x | Tier 1 or 2 | Same as 5.7 |
| MariaDB (all versions) | Tier 1 or 3 | Tier 2 is not used — MariaDB `statement.TIMER_WAIT` is stale during active execution; falls through to integer-second `processlist_time` |

See the Timer Wait Tiers section above for tier definitions.

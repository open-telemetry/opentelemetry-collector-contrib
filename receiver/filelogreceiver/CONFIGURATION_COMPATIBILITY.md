# Configuration compatibility review

This review maps [issue #52033](https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/52033)
to configuration compatibility evidence. It covers `FileLogConfig`, its embedded Stanza
configuration, and operator options used through `operators` or `header.metadata_operators`.
Runtime correctness, performance, and other graduation requirements are separate; the
receiver remains beta.

## Requirements and evidence

| Requirement | Implementation and evidence |
| --- | --- |
| Existing configurations remain usable across minor releases | `TestMinimalConfigCompatibility` decodes a minimal user configuration through the factory and compares it with a complete, independently decoded default baseline. `TestExplicitConfigCompatibility` loads non-default YAML through both receiver names and builds the operator pipeline. |
| New required options have sensible defaults | All existing optional defaults are pinned in `testdata/config_compatibility_defaults.yaml`. The minimal configuration must still build after adding options. `include` is the existing required selection of user files; inventing a default path would be unsafe. Requirements inside optional blocks apply only when that functionality is requested. |
| Migration instructions are in the component repository and linked from the README | [MIGRATION.md](MIGRATION.md) is linked in the README's configuration section. It covers the type alias, ordering limit, corrections to old documentation, and older configuration formats. |
| Deprecated configuration emits a WARN with a migration link | The shared file consumer warns when sorting uses an implicit limit. The receiver factory warns when the `filelog` alias is used. Receiver tests assert warning level, migration links, and absence of warnings for migrated configurations. |
| Old configuration is retained during the deprecation window | `filelog` remains a registered alias. Unset `top_n` still defaults to one with the default feature-gate state. Neither compatibility path nor warning is removed in this change; no removal is scheduled. The release and time requirements are documented in CONTRIBUTING.md and MIGRATION.md. |
| Feature gates do not silently add requirements to existing configurations | `TestOrderingConfigCompatibility` covers omitted, zero, positive, and negative ordering limits in both gate states. `TestGatedConfigCompatibility` covers deletion, header parsing, and mtime ordering with each gate enabled and disabled, including configurations that do not use the optional feature. |
| Migrating a renamed receiver preserves persisted offsets | `TestStorageAcrossReceiverTypeMigration` reads records, restarts under the new name, and rolls back to the alias using the same storage and instance name. It checks emitted record bodies, not merely successful construction. |
| Future security exceptions provide a migration path and upgrade period | The contributor policy explicitly requires both. No security exception is introduced by this change. |

## Configuration surface

The baseline pins the embedded file input, file consumer, matcher, attribute resolver,
multiline, whitespace, and adapter defaults. The explicit fixture exercises file selection,
reading and scheduling, metadata, lifecycle, ordering, header parsing, resources, storage,
retry, and operator pipelines through the public factory.

Mutually exclusive multiline modes, platform-specific owner and permission options, and
encoding, compression, truncation, and operator variants retain their receiver and Stanza
tests. Windows case-insensitive matching and Stanza emitter/scope gates affect runtime
behavior rather than required configuration keys. Release history and migration steps
are documented in [MIGRATION.md](MIGRATION.md).

## History reviewed

Moving fields into embedded Go types preserved their YAML names. The v0.149.0 rename
to `file_log` retained the `filelog` alias. In v0.159.0, omitted `top_n` retained its
default behind the disabled-by-default `filelog.requireExplicitTopN` gate; explicit
`top_n: 0` became all files. Pre-beta removal of `converter` (v0.64.0) and explicit
duration units (v0.60.0) are documented in [MIGRATION.md](MIGRATION.md).

## Maintaining the contract

Keep the baseline fixture and existing configuration fixtures when adding options. Add
tests for new options and any intended migration. Do not simply regenerate the baseline
to accept a changed default or delete old fixture keys: review such changes against the
deprecation policy first. The normal receiver tests run these checks in CI. Shared Stanza
changes must also run the receiver suite because they can change its configuration
without touching the receiver directory. The CI scope script already expands changed modules
to their transitive dependents, including this receiver when `pkg/stanza` changes.

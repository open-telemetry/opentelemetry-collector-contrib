# Configuration compatibility review

This review addresses the configuration requirements in
[issue #52033](https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/52033).
It covers `FileLogConfig` and the configuration it embeds from the Stanza file input,
file consumer, matcher, attribute resolver, multiline splitter, whitespace trimmer, and
receiver adapter. Operator-specific configuration is also subject to the compatibility
rules when used through `operators` or `header.metadata_operators`.

This is configuration graduation evidence, not a change to the receiver's overall beta
stability level. Runtime correctness, performance, and the other graduation requirements
are separate from this review.

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

The default fixture includes every field in the receiver's embedded configuration,
including inherited operator identity and output fields. The non-default fixture exercises
the following groups together through the public factory:

| Group | Fields |
| --- | --- |
| File selection | `include`, `exclude`, `exclude_older_than` |
| Reading and scheduling | `start_at`, `poll_interval`, `max_concurrent_files`, `max_batches`, `fingerprint_size`, `initial_buffer_size`, `max_log_size`, `max_log_size_behavior`, `encoding`, `force_flush_period`, `compression` |
| Multiline and whitespace | `multiline.line_start_pattern`, `multiline.line_end_pattern`, `multiline.omit_pattern`, `preserve_leading_whitespaces`, `preserve_trailing_whitespaces` |
| File metadata | `include_file_name`, `include_file_path`, `include_file_name_resolved`, `include_file_path_resolved`, `include_file_owner_name`, `include_file_owner_group_name`, `include_file_permissions`, `include_file_record_number`, `include_file_record_offset` |
| File lifecycle | `delete_after_read`, `acquire_fs_lock`, `file_cache_advise`, `skip_unmodified_files`, `polls_to_archive`, `on_truncate` |
| Ordering | `ordering_criteria.regex`, `group_by`, `top_n`, and `sort_by` entries with `sort_type`, `regex_key`, `ascending`, `layout`, `location` |
| Header metadata | `header.pattern`, `header.metadata_operators` |
| Pipeline and resources | `attributes`, `resource`, `operators`, `storage`, and all four `retry_on_failure` fields |

Mutually exclusive multiline modes, platform-specific owner/permission options, and
individual encoding, compression, truncation, and operator variants also have tests in
the receiver and shared Stanza packages. Those tests remain part of the validation scope;
one combined fixture cannot exercise incompatible modes simultaneously. The Windows
case-insensitive matching gate changes file matching rather than the accepted keys and
has platform-specific matcher tests. Stanza's emitter and scope gates change runtime
behavior, not which receiver configuration fields are required.

## History reviewed

The configuration history and changelog distinguish Go API refactoring from user YAML
changes. Moving fields into embedded types did not remove their YAML names. The receiver
type rename shipped in v0.149.0 with the old alias retained. The v0.159.0 ordering change
retained an omitted limit behind the disabled-by-default explicit-limit gate, but changed
explicit zero to mean all files. The migration guide now records both paths.

The removed `converter` block (v0.64.0) and explicit duration units (v0.60.0) predate beta;
their migration paths are documented without reintroducing obsolete implementations.
Recent options such as `max_log_size_behavior`, `on_truncate`, `file_cache_advise`, and
`skip_unmodified_files` have defaults that preserve the prior behavior.

## Maintaining the contract

Keep the baseline fixture and existing configuration fixtures when adding options. Add
tests for new options and any intended migration. Do not simply regenerate the baseline
to accept a changed default or delete old fixture keys: review such changes against the
deprecation policy first. The normal receiver tests run these checks in CI. Shared Stanza
changes must also run the receiver suite because they can change its configuration
without touching the receiver directory. The CI scope script already expands changed modules
to their transitive dependents, including this receiver when `pkg/stanza` changes.

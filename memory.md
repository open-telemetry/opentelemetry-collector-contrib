# Project Memory — Semconv Migration Work

Running log of decisions, conventions, and changes made on the semconv migration effort
(issue #22095). Companion to `SEMCONV_MIGRATION_TRACKER.md` — the tracker holds per-component
status; this file holds *why* we did things, working conventions, and a dated change log.

Last updated: 2026-10-24

---

## How to use this file

- **Tracker (`SEMCONV_MIGRATION_TRACKER.md`)** = current status of every component (done / in review / pending / do-not-touch).
- **This file (`memory.md`)** = decisions, gotchas, conventions, and a dated log of what we changed and why.
- Update the change log at the bottom whenever we finish a unit of work.

---

## Working conventions & durable decisions

1. **Read/match vs emit decides the strategy.**
   - Reading/matching incoming attributes (e.g. exporter checks span keys) → replace old constant with a **string literal or local const**; no feature gate needed (purely additive).
   - Emitting attributes (receiver/translator writes attrs into output) → add a **feature gate** (`EmitV1…` / `DontEmitV0…`) to control dropping the old attribute.

2. **Deprecated keys removed after their semconv version don't exist as constants in v1.40.0.**
   - e.g. `deployment.environment`, `db.system`, `exception.escaped` (post-v1.26.0); `peer.service`, `db.name`, `db.system` for servicegraph.
   - To drop the old import you must inline the value. **Maintainer preference (elasticsearch #49211): local consts with a deprecation comment**, not bare string literals.

3. **Dual-support for renamed keys.** When a key was renamed (not just removed), match BOTH old and new
   on read paths — e.g. add `peer.service.name` alongside `peer.service` and `db.system.name` alongside
   `db.system`. Put the current key first so it wins when spans contain both. Reviewers will ask for this
   if missing.

4. **Changelog (`.chloggen`) conventions.**
   - `change_logs: [user]` (not empty `[]` — empty is a common mistake that fails/needs fixing).
   - Keep notes to the user-facing change; drop internal detail like "removed X import" if a reviewer objects.
   - Feature-gate names ARE user-facing — name them in the entry so users know what to enable.
   - Validate with `make chlog-validate`.
   - IDE may flag chloggen YAML with `Missing property "file_format"` — that's a **false positive** (wrong schema); ignore it.

5. **Tooling gotchas.**
   - **WebFetch is unreliable for PR open/closed/merged state** — it misreported #49469 and #49301 in this project. **Verify merge state with git**, e.g.:
     `git fetch origin main && git merge-base --is-ancestor <sha> origin/main; echo $?` (0 = merged).
   - `--include=*.go` breaks under zsh (glob expansion). Quote it or use a different grep form.
   - Component identifier for chloggen = the component's `metadata.yaml` `type` prefixed by kind
     (e.g. dir `elasticsearchexporter`, type `elasticsearch` → `exporter/elasticsearch`).

6. **Don't duplicate work.** Before starting/continuing a component, check for an existing merged PR or an
   open PR by someone else (git history + PR search). Several components here were already done or claimed.

7. **Generated files must be regenerated, not hand-edited.**
   - `config.schema.yaml` is generated from the Go config struct's doc comments. When you change a
     config field's doc comment (e.g. a `Default is …` line), the schema goes stale.
   - CI runs `make checks`, which runs `make generate-schemas` then `git diff --exit-code` — so a stale
     schema **fails CI**. Regenerate; don't just edit the comment.
   - Regenerate one component only:
     `cd <component-dir> && go run go.opentelemetry.io/collector/cmd/schemagen@<ver> "$(pwd)" -o "$(pwd)"`
     (`<ver>` = `SCHEMAGEN_PKG` in `Makefile.Common`; whole-repo target is `make generate-schemas`).
   - Same principle for other generated artifacts (`generated_*.go`, `documentation.md`, `metadata.yaml`-derived
     files) — regenerate via mdatagen/`make generate`, don't hand-edit.

---

## Open PRs (ours) — 2026-07-27

| PR | Component | Approach | State |
|----|-----------|----------|-------|
| #49211 | exporter/elasticsearchexporter | Drop v1.26.0 import; deprecated keys → local consts; add `db.system.name` dual-support + test | In review |
| #49301 | pkg/translator/jaeger | Feature gates (`EmitV1HttpConventions`/`DontEmitV0HttpConventions`) + `getHTTPStatusCodeAttr` dual-read | In review (this branch); #49468 closed as duplicate |
| #49469 | connector/servicegraphconnector | Drop v1.25/1.28/1.38 imports; dual-support current `peer.service.name`/`db.namespace`/`db.system.name` and legacy `peer.service`/`db.name`/`db.system` | ✅ Maintainer follow-up addressed (2026-10-07); tests, fixtures, README, and changelog note updated. Pending review/merge; changelog still needs `change_logs: [user]` |
| (open) | receiver/awsxrayreceiver | 4 feature gates (HTTP + DB) | In review |

Closed as redundant: **#49360** (awsecscontainermetrics — superseded by merged #49078), **#49468** (jaeger — superseded by #49301).

---

## Do-not-touch / claimed by others

- faro (#45074), azureencoding (#47543), awslogsencoding (#47549) — feature-gate migrations in progress by others.
- exporter/awsxrayexporter — open draft #46537 (@paulojmdias, feature gates).
- hostmetrics / k8scluster / k8sattributes — blocked (#43869) or already on latest.

## Unclaimed & pickable

- **pkg/translator/zipkin** — genuinely available (no gates, no open PR), but complex (zipkinv1+v2 round-trip). Best remaining pickup.
- **internal/coreinternal/goldendataset** — low value (test infra); active merged series by others — coordinate before starting.

---

## Change log

### 2026-10-24

- **goldendataset external PR #52174:** reviewed the proposed v1.18.0/v1.26.0 to v1.40.0
  symbol updates. Moving `EnduserIDKey` and the unchanged resource keys is safe and does not need
  dual support.
- The PR keeps three legacy resource emissions through the v1.18.0 import rather than migrating
  them: `container.image.tag` at `resource_generator.go` lines 68 and 79, and `faas.id` at line 99
  (line numbers at PR head `859c542d8ec8ddac1077a4cc381df89d18c0b489`). This preserves the old
  output but does not provide the migration path requested in issue #22095.
- Recommended migration: feature-gated legacy-only -> dual-emission -> current-only behavior.
  `container.image.tag` (string) migrates to `container.image.tags` (string array), following the
  `awsecscontainermetricsreceiver` precedent. `faas.id` migrates to `cloud.resource_id`; the existing
  goldendataset `EmitV1FaaSConventions` / `DontEmitV0FaaSConventions` gates may be extended.
- Add resource-generator tests for all three gate states, including key presence and value types;
  the current test only checks that a non-empty resource has at least one attribute. No GitHub comment
  was posted or drafted because repository policy requires issue and PR discussion to be human-written.

### 2026-10-07

- **servicegraphconnector (#49469):** addressed feedback that `peer.service` was replaced by
  `peer.service.name`. `defaultPeerAttributes` now tries `peer.service.name` first and retains
  `peer.service` as a fallback. This is a read-only, additive compatibility change, so no feature gate is
  needed: current telemetry uses the new key, while spans from older SDKs and existing deployments still
  create virtual nodes instead of silently losing their peer value.
- Updated the virtual-node client test input to use `peer.service.name` and its expected metric label to
  `server_peer.service.name`. Added coverage for both peer keys and for current-key precedence. Updated
  the README default list/example and the chloggen note. The focused regression test and full
  `connector/servicegraphconnector` `go test ./...` suite pass (also confirmed by the user).
- **Remaining changelog cleanup:** `.chloggen/servicegraph-semconv-dual-support.yaml` is user-visible and
  should use `change_logs: [user]`, not `change_logs: []`.

### 2026-07-30

- **servicegraphconnector (#49469):** maintainer approved with one doc nit — the `DatabaseNameAttributes`
  comment in `config.go` still said the default was `{"db.name"}` but the implementation now defaults to
  `{"db.name", "db.namespace"}` (dual-support watch-item was addressed). Updated the comment, then
  **regenerated `config.schema.yaml`** (schemagen) so the derived description stays in sync — a bare
  comment edit would fail `make checks`. README (`Default: [db.name, db.namespace]`) was already correct;
  `config_test.go` left as-is (its `{"db.name"}` is an explicit test-config value, not the default).
  See new convention #7 (generated files).

### 2026-07-27

- **elasticsearchexporter (#49211):** dropped `semconv/v1.26.0` import; moved `deployment.environment`,
  `db.system`, `exception.escaped` to local consts with deprecation comments (per maintainer suggestion);
  added `db.system.name` → `span.db.type` dual-support; added `TestEncodeSpanECSModeDBSystemName`.
  Iterated `.chloggen/elasticsearchexporter-db-system-name.yaml` per reviewer feedback (removed import
  detail + deprecated-attrs sentence; kept `db.system.name` note; `change_logs: [user]`).
- **jaeger (#49301):** confirmed this branch IS #49301 (feature-gate impl), NOT merged to main yet.
  Established that PR description ("string literal, no gate") was stale — code correctly uses feature gates.
  Decided to close duplicate **#49468** (simpler approach). Flagged duplicate changelog:
  keep `.chloggen/migrateHTTP-45036.yaml`, delete `.chloggen/jaeger-semconv-dual-support.yaml`;
  `migrateHTTP-45036.yaml` still needs `change_logs: [user]` and `component: pkg/translator/jaeger`.
- **servicegraphconnector (#49469):** verified NOT migrated on main; reopened draft PR. Watch-item:
  add renamed `db.namespace`/`db.system.name` for dual-support.
- **awsecscontainermetrics:** confirmed done via merged #49078 (v1.42.0, two gates); our #49360 redundant.
- **redactionprocessor (#49467):** confirmed merged & clean (no semconv imports remain).
- **awsxrayexporter:** confirmed covered by others' open draft #46537 — do not touch.
- **zipkin / goldendataset:** corrected stale "in progress" note — zipkin is unclaimed; goldendataset is an active series by others.

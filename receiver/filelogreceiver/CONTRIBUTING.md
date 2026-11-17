# Contributing to the File Log Receiver

## Configuration compatibility

Configuration changes must follow the Collector's [component stability requirements](https://github.com/open-telemetry/opentelemetry-collector/blob/main/docs/component-stability.md#configuration-changes).
This includes changes inherited from `pkg/stanza`, such as file matching, input operators,
and retry settings, as well as changes in the receiver itself.

* Give new required options sensible defaults so existing configurations continue to work.
* Deprecate renamed or removed options in release N, retain them in N+1, and do not remove
  them or their warnings before both N+2 and six months after deprecation. They may be
  placed behind a feature gate in N+2. An option being removed may become non-operational
  in the release that deprecates it.
* Log a WARN message for deprecated configuration with a link to the corresponding
  section of [MIGRATION.md](MIGRATION.md). Record the first release with that warning
  and the migration steps there when introducing a deprecation.
* Test old configuration through the receiver factory, including omitted defaults,
  explicit values, warning messages, and both states of any compatibility feature gate.
* Once stable, maintain compatibility between minor versions. Critical security exceptions
  require a migration path and a reasonable upgrade period.
  Expiry of a deprecation period alone does not permit a breaking minor-version change:
  review alias removal and feature-gate default changes against this requirement too.

The receiver's stability level is managed separately from these configuration requirements.
See the [configuration compatibility review](CONFIGURATION_COMPATIBILITY.md) for the
audited surface and regression evidence. Preserve the default baseline and existing
configuration fixtures when introducing new options; changing them requires a compatibility
review, not just updating expected test values.

## Testing

### Benchmarks

To run benchmarks locally:
```bash
go test -bench=. -benchtime=1s
```

## Becoming a code owner

We are actively looking for new code owners for the File Log receiver.

Because the File Log receiver is essentially a thin wrapper over the [`pkg/stanza/fileconsumer`](../../pkg/stanza/fileconsumer/) package, becoming a code owner here means you will also co-own the `fileconsumer` package.
It is also tightly coupled to the broader [`pkg/stanza`](../../pkg/stanza/) module, so having a general understanding of Stanza is highly recommended.

### What You Need to Know

To successfully maintain these components, you will need a good grasp of the internals of the `fileconsumer` package. This includes:

* **File management basics:** How files are discovered, identified, and tracked.
* **Log reading mechanics:** When logs are read, how much data is read at once, and how many files are processed concurrently.
* **File tracking basics:** How known files are tracked across polls, how log rotation is handled.
* **Multiline processing:** How `multiline` option works.
* **Encoding:** How different character encodings are managed.
* **Compression:** How logs are read from compressed files.

Because the File Log receiver is frequently used with one or more Stanza operators, it is also beneficial to understand the broader mechanics of Stanza-based receivers:

* **Stanza entries vs. OTLP records:** What Stanza entries are and how they differ from OTLP log records.
* **The Stanza pipeline:** What operators are and how entries move from a Stanza input, through various transformers and parsers.
* **Conversion:** When and how Stanza entries are converted into OTLP log records.

### How to Build Your Knowledge

If you want to deepen your understanding of these areas, we highly encourage you to:

* Run the Collector and experiment with the File Log receiver using different configurations.
* Read through the underlying codebase.
* Review open issues and past pull requests.
* Ask questions in the CNCF Slack [#otel-collector](https://cloud-native.slack.com/archives/C01N6P7KR6W) channel, or create an issue in this repository.

### Expectations and How to Apply

Before applying to become a code owner, we ask that you build a track record of active, meaningful contributions to the component over a few weeks or months. This includes:

* Replying to and triaging issues.
* Reviewing pull requests from other community members.
* Contributing your own pull requests with enhancements or bug fixes.

While being a code owner doesn't come with strict quotas or explicit time commitments, we are looking for folks who can sustainably dedicate a little time each week to participate in issue discussions and review PRs.
To keep our community thriving, code owners may be asked to step down from the role if the component activity reports show little to no activity for a prolonged period.

**Ready to apply?** Create a pull request to add yourself as a code owner for both `receiver/filelog` and `pkg/stanza/fileconsumer` (example [PR](https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/38455/changes)). In the PR description, please highlight some of the meaningful contributions you've made recently.

If you are unsure whether you are ready, there is absolutely nothing wrong with reaching out to the existing code owners on the CNCF Slack. We are always happy to chat privately and share our thoughts before you open a public PR!

# Threat model

See
[Security Policy](https://github.com/open-telemetry/.github/blob/main/SECURITY.md)
for OpenTelemetry projects.

Additional context specific to this project can be found in the sections below.

## What this project does and where untrusted input enters
This is the contrib repository of the OpenTelemetry Collector: several hundred receivers, processors, exporters, connectors, extensions and shared packages that are used with the [core collector](https://github.com/open-telemetry/opentelemetry-collector). It contains hundreds of Go modules in one repository with no `go.work`. `cmd/otelcontribcol` is the contrib distribution, which is a local binary used for testing, and the official distributions are assembled elsewhere from these modules (https://github.com/open-telemetry/opentelemetry-collector-releases). Most components are built by operators into a custom collector with the builder, so which components exist in a running collector is up to the operator. Components have different stability levels and are maintained by different owners, as listed in each component's `metadata.yaml`. The OpenTelemetry Collector provides recommendations for deployment [configuration](https://opentelemetry.io/docs/security/config-best-practices/) and [hosting](https://opentelemetry.io/docs/security/hosting-best-practices/) best practices.

The trust boundary is authentication where a component can offer it. A receiver that serves HTTP or gRPC is meant to be deployed so that clients must authenticate before the collector accepts their data, using the authentication extensions (`extension/basicauthextension`, `extension/bearertokenauthextension`, `extension/oidcauthextension` and others) through the shared server settings of the core collector. An endpoint that can be configured with authentication and is intentionally left open by the operator is outside this model. Many receivers cannot authenticate their peers at all (raw TCP, UDP and syslog listeners, StatsD, and similar). For those, whoever can reach the listening address is an untrusted peer, and the operator is expected to restrict who can reach it.

Untrusted input:
- Anything a network peer can send to a receiver that listens on a network address, up to and including the authentication decision where there is one: the connection, TLS handshake, protocol framing, headers and credentials, and the parsing of the first request or datagram. For receivers with no authentication mechanism this is every byte they accept, so the parser is the attack surface. Examples are `receiver/syslogreceiver`, `receiver/statsdreceiver`, `receiver/webhookeventreceiver`, `receiver/prometheusremotewritereceiver`, `receiver/splunkhecreceiver`, `receiver/jaegerreceiver`, `receiver/zipkinreceiver`, `receiver/fluentforwardreceiver`, `receiver/influxdbreceiver`, `receiver/lokireceiver` and `receiver/carbonreceiver`.
- Responses from the systems a receiver scrapes or an exporter talks to (databases, cloud APIs, vendor backends, the Kubernetes API, a Prometheus target, an OpAMP server). The operator chooses the endpoint, but a response can still be hostile, for example a malformed body, an unexpected status or a very large payload.
- Content that a receiver reads from the local system and that other parties can influence: log lines read by `receiver/filelogreceiver` and `receiver/journaldreceiver`, container and pod metadata, process names and command lines, and similar. The file or source is chosen by the operator, but its contents are not trusted.

Trusted input:
- Telemetry from an authenticated client, and what then flows through the pipeline. An authenticated client is not an attacker here. Separate clients are still separate principals: one client's data, credentials and auth context must not reach another.
- Operator-controlled input. The collector configuration and everything it can reference: YAML files, `--set` flags, environment variables, and the config providers under `confmap/provider/`. This includes expressions and programs written in the configuration, such as OTTL statements (`pkg/ottl`, `processor/transformprocessor`, `processor/filterprocessor`), stanza operators (`pkg/stanza`) and SQL queries (`receiver/sqlqueryreceiver`). An attacker who can change the configuration already controls the collector.
- The set of components compiled into a binary. A component the operator compiled in is trusted code.

## Components that matter most / least
Most:
- Receivers with network listeners and no authentication, and the parsers behind them (see the list above), plus `pkg/stanza` and the receivers that use it for log parsing.
- Receivers, exporters and extensions that handle credentials: the authentication extensions (`extension/basicauthextension`, `extension/bearertokenauthextension`, `extension/oidcauthextension`, `extension/sigv4authextension` and others), the cloud and vendor exporters, and the secret-store config providers under `confmap/provider/`. The questions are whether a credential is validated before data is accepted, whether it can be logged or sent to the wrong destination, and whether one client can use another client's credentials.
- Extensions that serve internal state or accept control requests over HTTP, such as `extension/healthcheckextension` and `extension/pprofextension`, and `cmd/opampsupervisor`, which applies configuration received from its OpAMP server to the collector it manages.
- Components that read files, run commands or open connections based on values that can come from telemetry, for example exporters that build a file path, URL or query from an attribute, and processors that call external services.
- Shared packages that every component uses: `pkg/ottl`, `pkg/stanza`, `internal/coreinternal`, `internal/aws`, `internal/kubelet` and the other `internal/` and `pkg/` packages that handle parsing, TLS or credentials.
- Storage components (`extension/storage`) and exporters with persistent queues, where data is written to disk.

Least, still in scope but unlikely to matter: components that only act on data already inside the pipeline with no outside input (for example simple attribute and resource processors), the `*helper` packages other than their security-relevant paths, and `cmd/telemetrygen`, which only generates load from operator-supplied flags.

Generated code is in scope like any other code. Files named `generated_*` and the `internal/metadata` packages are overwritten by `cmd/mdatagen` from `metadata.yaml`, so a fix made only in the generated file is lost the next time it runs. When you find a flaw in one, say which generator or template produces it and patch that as well.

Out of scope: `testbed/`, `cmd/githubgen`, `cmd/codecovgen`, `cmd/golden`, `cmd/oteltestbedcol`, `internal/tools`, `examples/`, `docs/`, `.github/`, and test-support packages such as `pkg/pdatatest`, `pkg/golden` and the `*test` packages.

## How to exercise it
- The contrib collector is built at `/usr/local/bin/otelcontribcol`. `otelcontribcol validate --config <file>` and `otelcontribcol components` work without a backend. To run it, write a small configuration with the receiver you want to test and a `debug` exporter, for example a `webhookevent` or `statsd` receiver on a loopback address, then start it with `otelcontribcol --config <file>`.
- With the collector running, send input to the receiver with `curl`, `nc` or a small Go program, depending on the protocol.
- There is no network here and `GOPROXY=off`. Every module and the Go build cache were fetched during the image build.
- This is a multi-module repository: run tests from inside a module, for example `cd /src/receiver/webhookeventreceiver && go test ./...` or `cd /src/pkg/ottl && go test ./...`. `go test ./...` from `/src` matches no packages, because the root module has none.
- The test binaries of every module were compiled during the image build, but the tests were not run, and their compile output is in `/opt/oss-scanner-go-test.log`. Many component tests need a database, a container runtime or the Internet and will not pass here. Tests that bind to loopback work.
- Fuzz targets exist for some components. List them with `grep -rl 'func Fuzz' /src --include='*_test.go'`, and run one with, for example, `cd /src/receiver/webhookeventreceiver && go test -run '^$' -fuzz <FuzzName> -fuzztime 60s`.
- Certificates and keys under `testdata/` directories are throwaway fixtures for the tests.

## How you rate severity
Assess severity using the project guidelines (https://github.com/open-telemetry/sig-security/blob/main/security-response.md#severity) and use the project's published security model (https://github.com/open-telemetry/.github/blob/main/SECURITY.md).

## What threats are out of scope
- Anything that needs a valid client credential. Telemetry from an authenticated client is trusted, so a payload that crashes, stalls or exhausts memory in a processor or an exporter, or one that makes the collector mishandle its own data, is a bug and should be reported as one. The exception is a flaw that breaks isolation between clients.
- Anything that needs control over the configuration, environment variables or the compiled-in components, including OTTL statements, stanza operators and SQL queries written into the configuration.
- Behavior the operator opted into explicitly, such as `insecure: true`, `insecure_skip_verify`, or running an endpoint with no authentication configured where the component supports it. A receiver that has no authentication mechanism and is bound to a non-loopback address is the operator's deployment decision, but a parser flaw reachable through it is in scope.
- A debugging or diagnostic component printing telemetry or exposing internal state on its default `localhost` endpoint. Both are by design.
- Weaknesses in a dependency that cannot be reached from this code. If one can, show the call path.

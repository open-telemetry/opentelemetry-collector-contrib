# Local SNMP simulation

This example runs a real Collector with the SNMP receiver from your checkout.
[SNMP Simulator](https://docs.lextudio.com/snmpsim/quick-start) supplies polling
data and generates traps. [Net-SNMP](https://www.net-snmp.org/docs/man/snmptrap.html)
sends an Inform and waits for its acknowledgment. The Collector exports metrics
and logs as newline-delimited OTLP JSON files for inspection and assertions.

```text
Collector poll ── GET / GETNEXT / GETBULK ──> simulator 127.0.0.1:1161
Collector traps <── v1 / v2c / v3 traps ──── simulator notification module
Collector traps <── v2c Inform ──────────── Net-SNMP
                ── Response ─────────────> Net-SNMP
Collector pipelines ──> file exporter ──> metrics.json and logs.json
```

Both signals use the same `snmp/device` receiver ID in [collector.yaml](collector.yaml).
Its `poll` and `traps` configurations have separate endpoints and credentials.

## Prerequisites

- Linux or macOS for the commands and process lifecycle used by this example.
- A checkout containing the nested `poll` configuration and trap logs support.
  Build from this checkout; a released Collector may not include these changes.
- The Go version required by the repository's `go.mod` files, with access to
  download Go modules on the first build.
- Python 3.10 or newer, with virtual environment support. Python 3.12 was used
  for initial validation; the complete setup was also validated with Python 3.14.
- Net-SNMP commands `snmpget`, `snmpset`, `snmptrap`, and `snmpinform` on `PATH`.
- Free loopback UDP ports `1161` and `1620`.

Run as an ordinary user. Docker, real devices, privileged ports and MIB downloads
are unnecessary. The supplied communities and passwords are local example values.

## Set up and build

From the repository root:

```sh
cd receiver/snmpreceiver/examples/simulation
python3 -m venv .venv
.venv/bin/python -m pip install -r requirements.txt
.venv/bin/python build.py
```

[build.py](build.py) uses the repository's Collector builder and current component
versions to compile a small Collector with the local SNMP receiver and file
exporter. Local Contrib module replacements ensure it uses your checkout.
The binary is `.work/collector/snmp-simulation`. Its `build-info.json` records the
source checkout and commit. Build products, virtual environments and run results
are ignored by Git.

Dependencies are pinned in [requirements.txt](requirements.txt), including the
simulator's required PySMI import and SNMPv3 encryption support.

## Run the automated simulation

From this example directory:

```sh
.venv/bin/python run.py
```

The runner starts and stops its own simulator and Collector processes, checks
actual exported values, and exits with a nonzero status if an assertion fails.
It refuses to start if a required port is occupied. Stop any manually started
simulation before using it.

| Case | Assertion |
| --- | --- |
| Nested polling, v1 and v2c | Exact scalar and indexed table values over multiple collections |
| Nested polling, v3 | The same values with SHA authentication and AES privacy |
| Legacy polling | The same metrics and the legacy configuration warning |
| Mixed polling forms | Collector validation rejects nested and top-level polling settings together |
| Combined metrics and logs | Both pipelines run; polling continues after notifications |
| Simulator traps | v1, v2c and authenticated/encrypted v3 produce the expected structured log bodies and attributes |
| v2c Inform | Net-SNMP receives a Response; the expected log reaches the exporter |
| Rejected notifications | Wrong community Inform times out; wrong v3 authentication produces no exported log |
| Typed variable bindings | Preserve binding order, negative integers, Counter32, full Counter64, binary OctetString and numeric OIDs |
| Traps only | Notifications work without `poll`, a remote endpoint or metric mappings |
| Shutdown | Collector exits cleanly and releases the trap port |

Each run saves generated configurations, simulator and Collector logs, command
results, exported telemetry, and `summary.json` under `.work/runs/<run-id>/`.
The fixtures use constant values; this checks collection and mapping, not changing
counter rates. An Inform acknowledgment confirms local queue admission. The
exported log is a separate assertion of delivery through the local file exporter.

To validate a polling-only prerequisite checkout, build it separately and reuse
the polling subset of this runner:

```sh
.venv/bin/python build.py --source-root /path/to/polling-checkout \
  --output .work/poll-collector
.venv/bin/python run.py --poll-only \
  --collector .work/poll-collector/snmp-simulation
```

## Run polling and traps manually

Keep both processes running in separate terminals. Run all commands from this
example directory.

### 1. Start the simulator

```sh
.venv/bin/snmpsim-command-responder \
  --data-dir=./data --cache-dir=./.work/cache \
  --agent-udpv4-endpoint=127.0.0.1:1161 \
  --v3-user=sim-user --v3-auth-proto=SHA --v3-auth-key=authpass123 \
  --v3-priv-proto=AES --v3-priv-key=privpass123
```

[data/public.snmprec](data/public.snmprec) selects community `public` for v1/v2c
requests. [data/self.snmprec](data/self.snmprec) supplies the empty context used
for v3 polling. SNMP Simulator records use `OID|TYPE|VALUE`, with `67` for TimeTicks
and `65` for Counter32. The `notification` variation entries are SET triggers for
outgoing traps; polling them does not send notifications.

### 2. Start the Collector

```sh
mkdir -p .work
.work/collector/snmp-simulation validate --config collector.yaml
.work/collector/snmp-simulation --config collector.yaml
```

The metrics pipeline polls once per second. Check `.work/metrics.json` for:

| Metric | Attributes | Value |
| --- | --- | --- |
| `device.uptime` | none | `123456` hundredths of a second |
| `device.interface.in_octets` | `interface.index=if.1` | `1000` bytes |
| `device.interface.in_octets` | `interface.index=if.2` | `2000` bytes |

Verify the source independently in another terminal:

```sh
snmpget -m '' -v2c -c public -r 0 -t 2 -Otqv \
  127.0.0.1:1161 1.3.6.1.2.1.1.3.0
```

Expected output: `123456`.

### 3. Make the simulator send traps

```sh
# SNMPv2c coldStart trap.
snmpset -m '' -v2c -c public -r 0 -t 3 127.0.0.1:1161 \
  1.3.6.1.4.1.8072.9999.1.0 i 1
# SNMPv1 coldStart trap.
snmpset -m '' -v2c -c public -r 0 -t 3 127.0.0.1:1161 \
  1.3.6.1.4.1.8072.9999.3.0 i 1
# SNMPv3 coldStart trap with SHA authentication and AES privacy.
snmpset -m '' -v2c -c public -r 0 -t 3 127.0.0.1:1161 \
  1.3.6.1.4.1.8072.9999.4.0 i 1
```

These SET requests use v2c to control the simulator. Each trigger's fixture
selects the version of the outgoing trap. The logs pipeline should export one
record for each trigger in `.work/logs.json`, with the corresponding `snmp.version`,
`snmp.pdu.type=trap`, `snmp.trap.oid=1.3.6.1.6.3.1.1.5.1`, peer address and
uptime. The `sysName.0` binding contains `sim-v2c-trap`, `sim-v1-trap` or
`sim-v3-trap`. OctetString values appear as base64 `bytesValue` in OTLP JSON.
Use the exported records to verify delivery; a successful SET alone is insufficient.

### 4. Send an Inform and verify its acknowledgment

```sh
mkdir -p .work/net-snmp
export SNMP_PERSISTENT_DIR="$PWD/.work/net-snmp"
snmpinform -m '' -v2c -c public -r 0 -t 2 127.0.0.1:1620 \
  123456 1.3.6.1.6.3.1.1.5.1 \
  1.3.6.1.2.1.1.5.0 s net-snmp-inform
```

Exit status `0` means Net-SNMP received a Response. Check the exported log for
`snmp.pdu.type=inform` and the `net-snmp-inform` binding. `-r 0` disables retries
to avoid duplicate notifications in this example.

Repeat the command with `-c denied`: it should time out, return a nonzero status,
and produce no log. The automated runner also tests a v3 trap signed with the
wrong authentication password, then sends a valid Inform to confirm the listener
still works. It also sends a valid v3 trap from the same engine ID to verify that
the correct credentials are accepted.

## Troubleshooting and scope

- If startup reports a port conflict, stop the process using that port. The
  automated runner does not terminate processes it did not start.
- Unconfigured Redis/SQL variation modules can print load errors in SNMP Simulator;
  this fixture uses only its notification module.
- With the pinned simulator versions, a notification variation using
  `ntftype=inform` can block the simulator's request dispatcher. The supplied
  fixture uses that module for traps and Net-SNMP for Inform acknowledgment testing.
- Stop the manual Collector and simulator with Ctrl+C. Collector shutdown flushes
  the file exporter; rerunning the manual configuration truncates its output files.
- These tests establish delivery through a local Collector and file exporter.
  They do not validate real devices, remote exporter delivery, load/backpressure,
  packet loss or durable replay protection. SNMPv3 informs are unsupported.

For fixture format and notification parameters, see the simulator's
[quick start](https://docs.lextudio.com/snmpsim/quick-start) and
[notification module](https://docs.lextudio.com/snmpsim/documentation/simulation-with-variation-modules#notification-module).

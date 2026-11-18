# Local SNMP simulation

Run the Collector and [SNMP Simulator](https://docs.lextudio.com/snmpsim/quick-start)
to collect an uptime metric and an SNMPv2c trap. The Collector exports both to
local JSON files using [collector.yaml](collector.yaml).

## Prerequisites

- Linux or macOS with Python 3.10 or newer.
- A Collector Contrib executable supporting SNMP trap logs through the `traps` setting.
- The Net-SNMP `snmpset` command on `PATH`.
- Free loopback UDP ports `1161` and `1620`.

## 1. Set up

If you need to build the Collector, use the repository's
[standard build](../../../../CONTRIBUTING.md#local-testing) from the repository root:

```sh
make otelcontribcol
```

Install the simulator dependencies, also starting from the repository root:

```sh
cd receiver/snmpreceiver/examples/simulation
python3 -m venv .venv
.venv/bin/python -m pip install -r requirements.txt
mkdir -p .work/cache
```

Run the remaining commands from this example directory in each terminal.

## 2. Start the simulator

In the first terminal:

```sh
.venv/bin/snmpsim-command-responder \
  --data-dir=./data --cache-dir=./.work/cache \
  --agent-udpv4-endpoint=127.0.0.1:1161
```

[data/public.snmprec](data/public.snmprec) supplies the polling data for community
`public` and a SET trigger that sends a trap to the Collector.

## 3. Start the Collector

In the second terminal, use the binary built in step 1:

```sh
"../../../../bin/otelcontribcol_$(go env GOOS)_$(go env GOARCH)" \
  --config collector.yaml
```

If you already have a compatible Collector installed, replace the executable
path with its command, such as `otelcol-contrib`.

The Collector polls the simulator once per second and listens for traps on
`127.0.0.1:1620`.

## 4. Send a trap

In a third terminal, trigger an SNMPv2c coldStart trap from the simulator:

```sh
snmpset -m '' -v2c -c public -r 0 -t 3 127.0.0.1:1161 \
  1.3.6.1.4.1.8072.9999.1.0 i 1
```

## 5. Check the exported data

After a few seconds, inspect the latest exported records:

```sh
tail -n 1 .work/metrics.json
tail -n 1 .work/logs.json
```

- `metrics.json` should contain `device.uptime` with an `asInt` value of `123456`.
- `logs.json` should contain `snmp.pdu.type` set to `trap` and `snmp.trap.oid`
  set to `1.3.6.1.6.3.1.1.5.1` (coldStart).

These records confirm export through the local file exporter. Stop the Collector
and simulator with Ctrl+C when finished. Restarting the Collector overwrites
the output files.

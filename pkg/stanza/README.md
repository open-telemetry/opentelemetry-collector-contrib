# pkg/stanza

## Overview

This module contains functionality for reading and parsing logs from traditional log media.
Several of the collector's log receivers are based on this module.

### History

`stanza` was originally developed by observIQ as a standalone log agent. As a standalone agent, it had
capabilities for reading, parsing, and exporting logs. It was donated to the OpenTelemetry project in 2021.
Since then, it's been adapted to serve as the primary codebase for several log receivers in the OpenTelemetry Collector.

### Architecture

#### Data Model

`pkg/stanza` has an independent representation of OpenTelemetry's log data model where each individual log record
is modeled as an [`entry.Entry`](./docs/types/entry.md).

#### Operators

Functionality in this module is primarily organized into distinct [operators](./docs/operators/README.md). There are four types of operators:

- Input operators are the point of ingestion, e.g. reading from a file, or a TCP socket. These are anologous to receivers in the collector.
- Parser operators are responsible for extracting structured information from unstructured elements of a log record.
- Transform operators are responsible for modifying log records in some way, e.g. adding or removing a field.
- Output operators emit logs to an external destination. Most of these were removed because they were redundant with
  the OpenTelemetry Collector's exporters. A few were preserved as they are occasionally useful for debugging.

#### Operator Sequences

Operators are organized into [operator sequences](./docs/types/operators.md) to define the way in which logs should be read and parsed.
Operator sequences are somewhat similar to the OpenTelemetry Collector's notion of pipelines, but they are more flexible as data
may flow from any operator directly to any other operator, with a few natural restrictions.

#### Adapter

The `pkg/stanza/adapter` package is designed to facilitate integration of `pkg/stanza` operators into receivers.
It handles conversion between the local `entry.Entry` format and the OpenTelemetry Collector's `plog.Logs` format.

### Receiver Architecture

At a high level, each input operator is wrapped into a dedicated receiver.

Common functionality for all of these receivers is provided by the adapter package. This includes:

- The ability to define an arbitrary operator sequence, to allow users to fully interpret logs into the OpenTelemetry data model.
- A special `emitter` operator, combined with a `converter` which together act as a bridge from the operator sequence to the
  OpenTelemetry Collector's pipelines.

### Feature Gates

#### `stanza.synchronousLogEmitter`

The `stanza.synchronousLogEmitter` feature gate prevents possible data loss during an ungraceful shutdown of the collector by emitting logs in LogEmitter synchronously,
instead of batching the logs in LogEmitter's internal buffer. See related issue <https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/35456>.

LogEmitter is a component in Stanza that passes logs from Stanza pipeline to the collector's pipeline.
LogEmitter keeps an internal buffer of logs and only emits the logs as a single batch when the buffer is full (or when flush timeout elapses).
This was done in order to increase performance, as processing data in batches is much more performant than processing each entry separately.
However, this has the disadvantage of losing the data in the buffer in case of an ungraceful shutdown of the collector.
To prevent this, enable this feature gate to make LogEmitter synchronous, eliminating the risk of data loss.

Note that enabling this feature gate may have negative performance impact in some situations, see below.

The performance impact does not occur when using receivers based on Stanza inputs that support batching. Currently these are: File Log receiver. See caveat below.

The performance impact may be observed when using receivers based on Stanza inputs that do not support batching. Currently these are: Journald receiver, Named Pipe receiver, Syslog receiver, TCP Log receiver, UDP Log receiver, Windows EventLog receiver.

The caveat is that even when using a receiver that supports batching (like the File Log receiver), the performance impact may still be observed when additional operators are configured (see `operators` configuration option).
This is because Stanza transform operators currently don't support processing logs in batches, so even if the File Log receiver's File input operator creates a batch of logs,
the next operator in Stanza pipeline will split every batch into single entries.

The planned schedule for this feature gate is the following:

- Introduce as `Alpha` (disabled by default) in v0.122.0
- Move to `Beta` (enabled by default) after transform operators support batching and after all receivers that are selected to support batching support it

### FAQ

Q: Why don't we make every parser and transform operator into a distinct OpenTelemetry processor?

A: The nature of a receiver is that it reads data from somewhere, converts it into the OpenTelemetry data model, and emits it.
   Unlike most other receivers, those which read logs from traditional log media generally require extensive flexibility in the way that they convert an external format into the OpenTelemetry data model. Parser and transformer operators are designed specifically to provide sufficient flexibility to handle this conversion. Therefore, they are embedded directly into receivers
   in order to encapsulate the conversion logic and to prevent this concern from leaking further down the pipeline.

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

### FAQ

Q: Why don't we make every parser and transform operator into a distinct OpenTelemetry processor?

A: The nature of a receiver is that it reads data from somewhere, converts it into the OpenTelemetry data model, and emits it.
   Unlike most other receivers, those which read logs from traditional log media generally require extensive flexibility in the way that they convert an external format into the OpenTelemetry data model. Parser and transformer operators are designed specifically to provide sufficient flexibility to handle this conversion. Therefore, they are embedded directly into receivers
   in order to encapsulate the conversion logic and to prevent this concern from leaking further down the pipeline.

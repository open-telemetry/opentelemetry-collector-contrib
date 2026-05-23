# componentchecker

This package provides utilities for extracting and converting OpenTelemetry Collector components and modules into a format suitable for Datadog payloads.

## Overview

The `componentchecker` module is responsible for:

- Parsing the OpenTelemetry Collector configuration to identify all configured and active components (such as receivers, processors, exporters, extensions, and connectors).
- Collecting module information (including Go module path and version) for each component.
- Converting this information into a JSON payload format compatible with Datadog's requirements.

## Key Functions

- **PopulateFullComponentsJSON**: Extracts all components from the Collector's module information and configuration, and produces a comprehensive JSON structure describing each component, its type, kind, Go module, version, and whether it is configured.
- **PopulateActiveComponents**: Identifies the active components in the running Collector service, gathers their metadata, and prepares a list of service components for inclusion in Datadog fleet payloads.

## Usage

This module is intended to be used as part of the Datadog extension for the OpenTelemetry Collector, enabling enhanced observability and reporting of Collector internals to Datadog.

---

For more details, see the source code and function documentation in `componentchecker.go`.

# Compression configuration package

## Overview

This package provides a mechanism for configuring the static
compressors registered with gRPC for the Zstd compression library.
This enables OpenTelemetry Collector components to control all aspects
of Zstd configuration for both encoding and decoding.

## Usage

Embed the `EncoderConfig` or `DecoderConfig` object into the component
configuration struct.  Compression levels 1..10 are supported via
static compressor objects, registered with names "zstdarrow1",
"zstdarrow10" corresponding with Zstd level.  Level-independent
settings are modified using `SetEncoderConfig` or `SetDecoderConfig`.

For exporters, use `EncoderConfig.CallOption()` at the gRPC call site.

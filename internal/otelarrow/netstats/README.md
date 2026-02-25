# Network Statistics package

## Overview

This package provides general support for counting the number of bytes
read and written by an exporter or receiver component.  This package
specifically supports monitoring the compression rate achieved by
OpenTelemetry Protocol with Apache Arrow, but it can be easily adopted
for any gRPC-based component, for both unary and streaming RPCs.

## Usage

To create a network reporter, pass the exporter or receiver settings
to `netstats.NewExporterNetworkReporter` or
`netstats.NewExporterNetworkReporter`, then register with gRPC:

```
	dialOpts = append(dialOpts, grpc.WithStatsHandler(netReporter.Handler()))
```

Because OTel-Arrow supports the use of compressed payloads, configured
through Arrow IPC, it is necessary for the exporter and receiver
components to manually account for uncompressed payload sizes.

The `SizesStruct` supports recording either one or both of the
compressed and uncompressed sizes.  To report only uncompressed size
in the exporter case, for example:

```
	var sized netstats.SizesStruct
	sized.Method = s.method
	sized.Length = int64(uncompressedSize)
	netReporter.CountSend(ctx, sized)
```

Likewise, the receiver uses `CountRecv` with `sized.Length` set to
report its uncompressed size after OTel-Arrow decompression.

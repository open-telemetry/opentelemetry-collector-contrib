# Context (OTTL)

The Context exposes request-related data from the Go `context.Context` passed through the Collector, such as client information and incoming gRPC metadata. Use these paths via the top-level `context` segment, optionally followed by sub-segments.

## Paths

The following paths are supported.

| path                               | field accessed                                                                                             | type                 |
|------------------------------------|------------------------------------------------------------------------------------------------------------|----------------------|
| context.client.addr                | the remote address string from the client info                                                             | string               |
| context.client.metadata            | client metadata attached to the request via `go.opentelemetry.io/collector/client`                         | pcommon.Map          |
| context.client.metadata[""]        | the value for a specific metadata key. Returns nil if key missing                                          | pcommon.Slice or nil |
| context.client.auth.attributes     | map of all auth attributes extracted from `client.Info.Auth` with values stringified (JSON for non-string) | pcommon.Map          |
| context.client.auth.attributes[""] | specific auth attribute value as a string (JSON for non-string values). Missing keys return empty string   | string               |
| context.grpc.metadata              | incoming gRPC metadata from the context (if present)                                                       | pcommon.Map          |
| context.grpc.metadata[""]          | values slice for a specific incoming gRPC metadata key. Returns nil if key missing                         | pcommon.Slice or nil |

All setters for these paths return an error; the context is read-only within OTTL expressions.



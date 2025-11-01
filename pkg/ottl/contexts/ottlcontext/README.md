# Context (OTTL)

The Context exposes request-related data from the Go `context.Context` passed through the Collector, such as client information and incoming gRPC metadata. Use these paths via the top-level `context` segment, optionally followed by sub-segments.

## Paths

The following paths are supported.

| path                               | field accessed                                                                                                            | type                                                                    |
|------------------------------------|---------------------------------------------------------------------------------------------------------------------------|-------------------------------------------------------------------------|
| context.client.addr                | the remote address string from the client info                                                                            | string                                                                  |
| context.client.metadata            | client metadata attached to the request via `go.opentelemetry.io/collector/client`                                        | pcommon.Map                                                             |
| context.client.metadata[""]        | the value for a specific metadata key                                                                                     | string or nil                                                           |
| context.client.auth.attributes     | map of all auth attributes values extracted from `client.Info.Auth`. Unsupported value types are mapped as empty string   | pcommon.Map                                                             |
| context.client.auth.attributes[""] | the value for a specific auth attribute key                                                                               | string, bool, int64, float64, pcommon.Map, pcommon.Slice, []byte or nil |
| context.grpc.metadata              | incoming gRPC metadata from the context                                                                                   | pcommon.Map                                                             |
| context.grpc.metadata[""]          | values slice for a specific incoming gRPC metadata key                                                                    | string or nil                                                           |


All setters for these paths return an error; the context is read-only within OTTL expressions.



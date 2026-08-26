# Azure Traces transformation rules

## General transformation rules

Transformation of Azure Trace records happened based on Category defined in incoming record (`type` field) using mappings described in this document.
Mapping are defined to be OpenTelemetry SemConv compatible as much as possible.

If any of the expected field is not present in incoming JSON record or has an empty string value (i.e. "") - it will be ignored.

### Unknown/Unsupported Azure Resource Log record Category

At the moment only `AppAvailabilityResults`, `AppDependencies` and `AppRequests` are supported.
Other Categories from `microsoft.insights/components` Azure Resources, like `AppEvents`, `AppExceptions`, `AppTraces`, etc. - are not supported as are they missing required fields (like `Id` or `OperationId` ) and looks more like Logs/Metrics.

## Base Span properties rules

| OpenTelemetry Span Property | Azure Trace Field       | Note |
|-----------------------------|-------------------------|------|
| TraceID                     | OperationId             | If `OperationId` is invalid or empty - whole record is ignored with warning logged |
| SpanID                      | Id                      | If `Id` equal to `OperationId` (i.e. 16 bytes length) - `SpanID` is set from first 8 bytes of `Id` field, if invalid or empty - whole record is ignored with warning logged |
| ParentSpanID                | ParentId                | If `ParentId` equal to `OperationId` (i.e. 16 bytes length) or empty or invalid - no `ParentSpanID` is set (i.e. assume that it's Root Span) |
| SpanKind                    | multiple                | calculated based on different rules for each category, see below |
| SpanName                    | Name, OperationName     | If `Name` empty or not set - `OperationName` field is used |
| StartTimestamp              | Time                    | If `Time` is invalid or empty - whole record is ignored with warning logged |
| EndTimestamp                | Time + DurationMs       | If `Time` is invalid or empty - whole record is ignored with warning logged |
| SpanStatusCode              | Success                 | Set to `Error` if `Success`=`false`, otherwise - `Unset` |

## SpanKind calculation rules

By default SpanKind is set to `Internal` as it is default value according to OpenTelemetry SemConv.

| Azure Category          | Rule                                  | OpenTelemetry SpanKind |
|-------------------------|---------------------------------------|------------------------|
| AppAvailabilityResults  | default                               | Internal |
| AppRequests             | default                               | Internal |
| AppRequests             | `Url` is set                          | Server   |
| AppDependencies         | default                               | Internal |
| AppDependencies         | `DependencyType` == `InProc`          | Internal |
| AppDependencies         | `DependencyType` starts with `Queue`  | Producer |
| AppDependencies         | `Data` is set                         | Client   |

## Common fields, available in all Categories

| Azure                     | OpenTelemetry                                 | OpenTelemetry Scope |
|---------------------------|-----------------------------------------------|---------------------|
| AppRoleInstance           | service.instance.id                           | Resource Attribute |
| AppRoleName               | service.name                                  | Resource Attribute |
| AppVersion                | service.version                               | Resource Attribute |
| ClientBrowser             | user_agent.original                           | Span Attribute |
| ClientCity                | geo.locality.name                             | Span Attribute |
| ClientCountryOrRegion     | geo.country.name                              | Span Attribute |
| ClientIP                  | client.address + client.port                  | Span Attribute |
| ClientModel               | device.model.name                             | Span Attribute |
| ClientOS                  | user_agent.os.name                            | Span Attribute |
| ClientStateOrProvince     | geo.state.name                                | Span Attribute |
| ClientType                | device.type                                   | Span Attribute |
| Properties                | copied as is to Span Attributes               | Span Attribute |
| resourceId                | cloud.resource_id                             | Resource Attribute |
| SDKVersion                | telemetry.sdk.name + telemetry.sdk.version    | Resource Attribute |
| SessionId                 | session.id                                    | Span Attribute |
| Success                   | -                                             | - |
| Type                      | azure.category                                | Span Attribute |
| UserId                    | user.id                                       | Span Attribute |

## AppAvailabilityResults specific fields

| Azure     | OpenTelemetry             | OpenTelemetry Scope |
|-----------|---------------------------|---------------------|
| Location  | cloud.region              | Resource Attribute |
| Message   | SpanStatusMessage         | Span |

## AppDependencies specific fields

NOTE: If `Data` field contains valid URL - it will be set to `url.full` attribute with parsed `url.scheme`, `url.domain`, `url.fragment`, `url.query`, `url.path` and `url.port`.

| Azure             | OpenTelemetry                         | OpenTelemetry Scope |
|-------------------|---------------------------------------|---------------------|
| ResultCode        | http.response.status_code (if data is URL) | Span Attribute |
| Data              | azure.dependency.data OR set of url.* span attributes  | Span Attribute |
| DependencyType    | azure.dependency.type                 | Span Attribute |
| Target            | azure.dependency.target               | Span Attribute |

## AppRequests specific fields

| Azure         | OpenTelemetry                 | OpenTelemetry Scope |
|---------------|-------------------------------|---------------------|
| ResultCode    | http.response.status_code (if URL is present) | Span Attribute |
| Source        | azure.request.source       | Span Attribute |
| URL           | `url.full` with parsed `url.scheme`, `url.domain`, `url.fragment`, `url.query`, `url.path` and `url.port`. If unparsable - only `url.original` | Span Attribute |

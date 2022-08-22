# Syslog Exporter

| Status                   |           |
| ------------------------ |-----------|
| Stability                | [alpha]   |
| Supported pipeline types | logs      |
| Distributions            | [contrib] |

This exporter will send logs to the Syslog server in rfc5424 format.
It will be possible to send logs by TCP and UDP.

## Configuration options
- `endpoint` - Endpoint where to send the data. Error if not set. Could be set with SYSLOG_HOST env variable
- `net_protocol` - Defines which network protocol to use. (By default TCP)
- `sd_config` - Custom configuration of structured data
- `sd_config.common_sdid` - Defines which SDID should be used for attributes. (meta by default)
- `sd_config.trace_sdid` - Defines which SDID should be used for trace_id and span_id attributes. (meta by default)
- `sd_config.mapping_sdid` - Key value list. Defines custom SDID for the specific attribute names.   
   For example in the config all attributes will have SDID user@12345 but tag attribute will have metadata@12345 SDID
- `sd_config.static_sd` - Defines structured data that will be added to each log entry. In format: SDID: key:value

### Structured data
Exporter will add resources and log records attributes as structured data  
Id of the sd-element can be rewritten by configuration options (meta by default)  
Attributes from the resource and log records will be merged since each Syslog record is an independent entity.  
service.name and host.hostname attributes will be excluded from structured data and used as HOSTNAME and APPNAME in the message.  

## Example

```yaml
syslog:
    endpoint: localhost:5552
    net_protocol: tcp
    sd_config:
      common_sdid: user@12345
      trace_sdid: opentelemetry@12345
      mapping_sdid:
        tag: metadata@12345
      static_sd:
        location@12345:
          env: prod
          loc: bcn
        stats@51719:
          proc: "1"
```
[beta]:https://github.com/open-telemetry/opentelemetry-collector#alpha
[contrib]:https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib
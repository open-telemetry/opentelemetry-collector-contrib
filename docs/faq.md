# Frequently asked questions

## How do I configure the agent?
The agent is configured using a YAML file. Use the `--config` flag to tell stanza where to find this file. 

This file defines a collection of operators that make up a `pipeline`. Each operator possesses a `type` field, and can optionally be given an `id` as well.

```yaml
pipeline:
  - type: udp_input
    listen_address: :5141

  - type: syslog_parser
    parse_from: message
    protocol: rfc5424

  - type: elastic_output
```

## Does Stanza support HTTP proxies?

Yes. Read about the details [here](/docs/proxy.md).

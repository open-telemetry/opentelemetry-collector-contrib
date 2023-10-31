# MockServer

This is explicitly a test client for the `vcenterreceiver`.

It emulates a vCenter server with this Product Information:

```json
      "name": "VMware vCenter Server",
      "fullName": "VMware vCenter Server 7.0.2 build-19272235",
      "vendor": "VMware, Inc.",
      "version": "7.0.2",
      "build": "19272235",
      "localeVersion": "INTL",
      "localeBuild": "000",
      "osType": "linux-x64",
      "productLineId": "vpx",
      "apiType": "VirtualCenter",
      "apiVersion": "7.0.2.0",
      "licenseProductName": "VMware VirtualCenter Server",
      "licenseProductVersion": "7.0"
```

## Method of recording

[mitmproxy](https://docs.mitmproxy.org/stable/) was used to record the scraper via reverse proxy recording.

The command to run `mitmproxy`:

```sh
mitmproxy -p 55626 --mode=reverse:https://<vcenter-hostname>
```

And then running the receiver against the proxy.

```yaml
receivers:
  vcenter:
    endpoint: https://localhost:56626
    username: "otelu"
    password: "otelp"
    tls:
      insecure: false
      insecure_skip_verify: true

service:
  pipelines:
    metrics:
      receivers: [vcenter]
```

Note govmomi uses cookie based authentication. Because of this the environment variable `GOVMOMI_INSECURE_COOKIES=true` may need to be set to let the receiver collect.

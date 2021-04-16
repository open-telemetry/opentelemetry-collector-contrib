# Logzio Exporter

This exporter supports sending trace data to [Logz.io](https://www.logz.io)

The following configuration options are supported:

* `account_token` (Required): Your logz.io account token for your tracing account.
* `metrics_token` (Required): Your logz.io account token for your metrics account.
* `region` (Optional): Your logz.io account [region code](https://docs.logz.io/user-guide/accounts/account-region.html#available-regions). Defaults to `us`. Required only if your logz.io region is different than US.
* `custom_endpoint` (Optional): Custom endpoint, mostly used for dev or testing. This will override the region parameter.

Example:

```yaml
exporters:
  logzio:
    account_token: "youLOGZIOtraceTOKEN"
    metrics_token: "youLOGZIOmetricsTOKEN"
    region: "eu"
```

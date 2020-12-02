# Logzio Exporter

This exporter supports sending trace data to [Logz.io](https://www.logz.io)

The following configuration options are supported:

* `account_token` (Required): Your logz.io account token for your tracing account.
* `region` (Optional): Your logz.io account [region code](https://docs.logz.io/user-guide/accounts/account-region.html#available-regions). Defaults to `us`. Required only if your logz.io region is different than US.
* `custom_listener_address` (Optional): Custom traces endpoint, for dev. This will override the region parameter.

Example:

```yaml
exporters:
  logzio:
    account_token: "youLOGZIOaccountTOKEN"
    region: "eu"
```

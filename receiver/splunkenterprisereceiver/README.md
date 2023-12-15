# Splunk Enterprise Receiver

The Splunk Enterprise Receiver is a pull based tool which enables the ingestion of performance metrics describing the operational status of a user's Splunk Enterprise deployment to an appropriate observability tool.
It is designed to leverage several different data sources to gather these metrics including the [introspection api endpoint](https://docs.splunk.com/Documentation/Splunk/9.1.1/RESTREF/RESTintrospect) and serializing
results from ad-hoc searches. Because of this, care must be taken by users when enabling metrics as running searches can effect your Splunk Enterprise Deployment and introspection may fail to report for Splunk
Cloud deployments. The primary purpose of this receiver is to empower those tasked with the maintenance and care of a Splunk Enterprise deployment to leverage opentelemetry and their observability toolset in their
jobs.

## Configuration

The following settings are required, omitting them will either cause your receiver to fail to compile or result in 4/5xx return codes during scraping.

* `basicauth` (from [basicauthextension](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/extension/basicauthextension)): A configured stanza for the basicauthextension.
* `auth` (no default): String name referencing your auth extension.
* `endpoint` (no default): your Splunk Enterprise host's endpoint.

The following settings are optional:

* `collection_interval` (default: 10m): The time between scrape attempts.
* `timeout` (default: 60s): The time the scrape function will wait for a response before returning empty.

Example:

```yaml
extensions:
    basicauth/client:
        client_auth:
            username: admin
            password: securityFirst

receivers:
    splunkenterprise:
        auth: basicauth/client
        endpoint: "https://localhost:8089"
        timeout: 45s
```

For a full list of settings exposed by this receiver please look [here](./config.go) with a detailed configuration [here](./testdata/config.yaml).

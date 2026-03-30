# Google SecOps Exporter

This exporter facilitates the sending of logs to [Google SecOps](https://cloud.google.com/security/products/security-operations) (previously Chronicle), a security analytics platform provided by Google. It is designed to integrate with OpenTelemetry collectors to export logs Google SecOps.

## Supported APIs

This exporter supports sending logs to Google SecOps using either of the following APIs
- [Chronicle API](https://docs.cloud.google.com/chronicle/docs/reference/ingestion-methods) (Preferred)
- [Backstory Ingestion API](https://docs.cloud.google.com/chronicle/docs/reference/ingestion-api)

## How It Works

1. The exporter uses the configured credentials to authenticate with the Google Cloud services.
2. Logs are marshalled into the format expected by Google SecOps.
3. Logs are imported into Google SecOps using the appropriate endpoint.

# AWS X-Ray Proxy

##
The AWS X-Ray proxy accepts requests without any authentication of AWS signatures applied and forwards them to the
AWS X-Ray API, applying authentication and signing. This allows applications to avoid needing AWS credentials to enable
X-Ray, instead configuring the AWS X-Ray exporter and/or proxy in the OpenTelemetry collector and only providing the
collector with credentials.

Currently, only 
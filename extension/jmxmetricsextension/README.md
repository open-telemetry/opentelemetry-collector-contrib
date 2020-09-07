# JMX Metrics Extension

### Overview

The JMX Metrics Extension will work in conjunction with the upcoming [OpenTelemetry
JMX Metric Gatherer](https://github.com/open-telemetry/opentelemetry-java-contrib/pull/4) to
report metrics from a target MBean server using your custom `otel` helper-utilizing Groovy script.

Status: alpha

### Details

This extension will launch a child JRE process running the JMX Metric Gatherer configured with your specified JMX
connection information and target Groovy script.  It can report metrics to an existing otlp or prometheus metric
receiver in your pipeline.

# Configuration

Note: this extension is in alpha and functionality and configuration fields are subject to change.  It is currently
partially implemented.

Example configuration:

```yaml
extensions:
  jmx_metrics:
    service_url: service:jmx:rmi:///jndi/rmi://<my-jmx-host>:<my-jmx-port>/jmxrmi
    groovy_script: /opt/my/groovy.script
    interval: 10s
    username: my_jmx_username
    # determined by the environment variable value
    password: $MY_JMX_PASSWORD
```

### service_url

The JMX Service URL the Metric Gatherer's JMX client should use.

Corresponds to the `otel.jmx.service.url` property.

_Required._

### groovy_script

The path of the Groovy script the Metric Gatherer should run.

Corresponds to the `otel.jmx.groovy.script` property.

_Required._

### interval (default: 10s)

The interval time for the Groovy script to be run and metrics exported.  Will be converted to milliseconds.

Corresponds to the `otel.jmx.interval.milliseconds` property.

### username

The username to use for JMX authentication.

Corresponds to the `otel.jmx.username` property.

### password

The password to use for JMX authentication.

Corresponds to the `otel.jmx.password` property.

### otlp_timeout (default: 1s)

The otlp exporter request timeout.  Will be converted to milliseconds.

Corresponds to the `otel.otlp.metric.timeout` property.

### otlp_headers

The headers to include in otlp metric submission requests.

Corresponds to the `otel.otlp.metadata` property.

### keystore_path

The keystore path is required if SSL is enabled on the target JVM.

Corresponds to the `javax.net.ssl.keyStore` property.

### keystore_password

The keystore file password if required by SSL.

Corresponds to the `javax.net.ssl.keyStorePassword` property.

### keystore_type

The keystore type if required by SSL.

Corresponds to the `javax.net.ssl.keyStoreType` property.

### truststore_path 

The truststore path if the SSL profile is required.

Corresponds to the `javax.net.ssl.trustStore` property.

### truststore_password

The truststore file password if required by SSL.

Corresponds to the `javax.net.ssl.trustStorePassword` property.

### remote_profile

Supported JMX remote profiles are TLS in combination with SASL profiles: SASL/PLAIN, SASL/DIGEST-MD5 and SASL/CRAM-MD5.
Should be one of: `"SASL/PLAIN"`, `"SASL/DIGEST-MD5"`, `"SASL/CRAM-MD5"`, `"TLS SASL/PLAIN"`, `"TLS SASL/DIGEST-MD5"`,
or `"TLS SASL/CRAM-MD5"`, though no enforcement is applied.

Corresponds to the `otel.jmx.remote.profile` property.

### realm

The realm, as required by remote profile SASL/DIGEST-MD5.

Corresponds to the `otel.jmx.realm` property.

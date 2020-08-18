# JMX Metrics Receiver

### Overview

The JMX Metrics Receiver will work in conjunction with the upcoming OpenTelemetry JMX Metric Gatherer to
report metrics from a target MBean server using your custom `otel` helper-utilizing Groovy script.

Status: alpha

### Details

This receiver will launch a child JRE process running the JMX Metric Gatherer configured with your specified JMX
connection information and target Groovy script.  It can report metrics to an existing otlp metric receiver specified
by the `collector_endpoint` configuration option.

# Configuration

Note: this receiver is in alpha and functionality and configuration fields are subject to change.  It is currently
partially implemented.

Example configuration:

```yaml
receivers:
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

Corresponds to the `otel.jmx.metrics.service.url` property.

_Required._

### groovy_script

The path of the Groovy script the Metric Gatherer should run.

Corresponds to the `otel.jmx.metrics.groovy.script` property.

_Required._

### collector_endpoint

The endpoint of an existing otlp receiver to report metrics to.  If none is specified, one will implicitly be created.

Corresponds to the `otel.jmx.metrics.exporter.endpoint` property.

### interval (default: 10s)

The interval time for the Groovy script to be run and metrics exported.  Will be converted to milliseconds.

Corresponds to the `otel.jmx.metrics.interval.milliseconds` property.

### keystore_path

The keystore path is required if SSL is enabled on the target JVM.

Corresponds to the `otel.jmx.metrics.keystore.path`

### keystore_password

The keystore file password if required by SSL.

Corresponds to the `otel.jmx.metrics.keystore.password`

### keystore_type

The keystore type if required by SSL.

Corresponds to the `otel.jmx.metrics.keystore.type`

### truststore_path 

The truststore path if the SSL profile is required.

Corresponds to the `otel.jmx.metrics.truststore.path`

### truststore_password

The truststore file password if required by SSL.

Corresponds to the `otel.jmx.metrics.truststore.password`

### remote_profile

Supported JMX remote profiles are TLS in combination with SASL profiles: SASL/PLAIN, SASL/DIGEST-MD5 and SASL/CRAM-MD5.
Should be one of: `"SASL/PLAIN"`, `"SASL/DIGEST-MD5"`, `"SASL/CRAM-MD5"`, `"TLS SASL/PLAIN"`, `"TLS SASL/DIGEST-MD5"`,
or `"TLS SASL/CRAM-MD5"`, though no enforcement is applied.

Corresponds to the `otel.jmx.metrics.remote.profile`

### realm

The realm, as required by remote profile SASL/DIGEST-MD5.

Corresponds to the `otel.jmx.metrics.realm`

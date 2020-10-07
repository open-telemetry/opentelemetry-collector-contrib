# JMX Metrics Extension

### Overview

The JMX Metrics Extension will work in conjunction with the [OpenTelemetry JMX Metric Gatherer](https://github.com/open-telemetry/opentelemetry-java-contrib/blob/master/contrib/jmx-metrics/README.md)
to report metrics from a target MBean server using a built-in or your custom `otel` helper-utilizing
Groovy script.

Status: alpha

### Details

This extension will launch a child JRE process running the JMX Metric Gatherer configured with your specified JMX
connection information and target Groovy script.  It can report metrics to an existing otlp or prometheus metric
receiver in your pipeline.  In order to use you will need to download the most [recent release](https://oss.jfrog.org/artifactory/oss-snapshot-local/io/opentelemetry/contrib/opentelemetry-java-contrib-jmx-metrics/)
of the JMX Metric Gatherer jar and configure the extension with its path.  It is assumed that the JRE is
available on your system.

# Configuration

Note: this extension is in alpha and functionality and configuration fields are subject to change.

Example configuration:

```yaml
extensions:
  jmx_metrics:
    jar_path: /opt/opentelemetry-java-contrib-jmx-metrics.jar
    service_url: service:jmx:rmi:///jndi/rmi://<my-jmx-host>:<my-jmx-port>/jmxrmi
    groovy_script: /opt/my/groovy.script
    interval: 10s
    exporter: otlp
    otlp_endpoint: mycollectorotlpreceiver:55680
    username: my_jmx_username
    # determined by the environment variable value
    password: $MY_JMX_PASSWORD
```

### jar_path (default: `/opt/opentelemetry-java-contrib-jmx-metrics.jar`)

The path for the JMX Metric Gatherer uber jar to run.

### service_url

The JMX Service URL the Metric Gatherer's JMX client should use.

Corresponds to the `otel.jmx.service.url` property.

_Required._

### target_system

The built-in target system metric gatherer script to run.

Corresponds to the `otel.jmx.target.system` property.

One of `groovy_script` or `target_system` is _required_.  Both cannot be specified at the same time.

### groovy_script

The path of the Groovy script the Metric Gatherer should run.

Corresponds to the `otel.jmx.groovy.script` property.

One of `groovy_script` or `target_system` is _required_.  Both cannot be specified at the same time.

### interval (default: `10s`)

The interval time for the Groovy script to be run and metrics exported.

Corresponds to the `otel.jmx.interval.milliseconds` property.

### username

The username to use for JMX authentication.

Corresponds to the `otel.jmx.username` property.

### password

The password to use for JMX authentication.

Corresponds to the `otel.jmx.password` property.

### exporter (default: `otlp`)

The metric exporter to use, which matches a desired existing `otlp` or `prometheus` receiver type.

Corresponds to the `otel.exporter` property.

### otlp_endpoint (default: `localhost:55680`)

The otlp exporter endpoint to submit metrics.  Must coincide with an existing `otlp` receiver `endpoint`.

Corresponds to the `otel.otlp.endpoint` property.

### otlp_timeout (default: `5s`)

The otlp exporter request timeout.

Corresponds to the `otel.otlp.metric.timeout` property.

### otlp_headers

The headers to include in otlp metric submission requests.

Corresponds to the `otel.otlp.metadata` property.

### prometheus_host (default: `localhost`)

The JMX Metric Gatherer prometheus server host interface to listen to.

Corresponds to the `otel.prometheus.host` property.

### prometheus_port (default: `9090`)

The JMX Metric Gatherer prometheus server port to listen to.

Corresponds to the `otel.prometheus.port` property.

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

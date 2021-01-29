# JMX Receiver

### Overview

The JMX Receiver will work in conjunction with the [OpenTelemetry JMX Metric Gatherer](https://github.com/open-telemetry/opentelemetry-java-contrib/blob/main/contrib/jmx-metrics/README.md)
to report metrics from a target MBean server using a built-in or your custom `otel` helper-utilizing
Groovy script.

Status: alpha

### Details

This receiver will launch a child JRE process running the JMX Metric Gatherer configured with your specified JMX
connection information and target Groovy script.  It then reports metrics to an implicitly created OTLP receiver.
In order to use you will need to download the most [recent release](https://oss.jfrog.org/artifactory/oss-snapshot-local/io/opentelemetry/contrib/opentelemetry-java-contrib-jmx-metrics/)
of the JMX Metric Gatherer JAR and configure the receiver with its path.  It is assumed that the JRE is
available on your system.

# Configuration

Note: this receiver is in alpha and functionality and configuration fields are subject to change.

Example configuration:

```yaml
receivers:
  jmx:
    jar_path: /opt/opentelemetry-java-contrib-jmx-metrics.jar
    endpoint: my_jmx_host:12345
    target_system: jvm
    collection_interval: 10s
    # optional: the same as specifying OTLP receiver endpoint.
    otlp:
      endpoint: mycollectorotlpreceiver:55680
    username: my_jmx_username
    # determined by the environment variable value
    password: $MY_JMX_PASSWORD
```

### jar_path (default: `/opt/opentelemetry-java-contrib-jmx-metrics.jar`)

The path for the JMX Metric Gatherer uber JAR to run.

### endpoint
The [JMX Service URL](https://docs.oracle.com/javase/8/docs/api/javax/management/remote/JMXServiceURL.html) or host
and port used to construct the Service URL the Metric Gatherer's JMX client should use. Value must be in the form of
`service:jmx:<protocol>:<sap>` or `host:port`. Values in `host:port` form will be used to create a Service URL of
`service:jmx:rmi:///jndi/rmi://<host>:<port>/jmxrmi`.

When in or coerced to `service:jmx:<protocol>:<sap>` form, corresponds to the `otel.jmx.service.url` property.

_Required._

### target_system

The built-in target system metric gatherer script to run.

Corresponds to the `otel.jmx.target.system` property.

One of `groovy_script` or `target_system` is _required_.  Both cannot be specified at the same time.

### groovy_script

The path of the Groovy script the Metric Gatherer should run.

Corresponds to the `otel.jmx.groovy.script` property.

One of `groovy_script` or `target_system` is _required_.  Both cannot be specified at the same time.

### collection_interval (default: `10s`)

The interval time for the Groovy script to be run and metrics to be exported by the JMX Metric Gatherer within the persistent JRE process.

Corresponds to the `otel.jmx.interval.milliseconds` property.

### username

The username to use for JMX authentication.

Corresponds to the `otel.jmx.username` property.

### password

The password to use for JMX authentication.

Corresponds to the `otel.jmx.password` property.

### otlp.endpoint (default: `0.0.0.0:<random open port>`)

The otlp exporter endpoint to which to listen and submit metrics.

Corresponds to the `otel.exporter.otlp.endpoint` property.

### otlp.timeout (default: `5s`)

The otlp exporter request timeout.

Corresponds to the `otel.exporter.otlp.metric.timeout` property.

### otlp.headers

The headers to include in otlp metric submission requests.

Corresponds to the `otel.exporter.otlp.metadata` property.

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

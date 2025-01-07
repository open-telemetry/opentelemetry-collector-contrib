# Build A Secure Trace Collection Pipeline 

Implementing robust security measures is essential for any tracing ingestion service and infrastructure. This is an illustrative example of a secure setup encompassing the following features for trace ingestion:

- Data Encryption with OTLP receiver supporting TLS. Transport Layer Security (TLS) is employed to encrypt traces in transit between the application and the OpenTelemetry (OTLP) endpoint, fortifying data security.
- Client Authentication via [Envoy](https://www.envoyproxy.io/docs/envoy/latest/start/start), a high-performance proxy. 
Even though we can configure OTLP receiver with mTLS for client authentication, authorization is not supported by OpenTelemetry Collector. This is one of the reasons that we use Envoy for client authentication. It allows us to easily add authorization, ensuring that only authorized clients can submit traces to the ingestion service.

In this example, we also include a test via telementrygen: a tool provided from this repository for generating synthetic telemetry data, which helps verify the security features and overall functionality of the set up. 


## Data Encryption via TLS
The OpenTelemetry Collector has detailed [documentation](https://github.com/open-telemetry/opentelemetry-collector/blob/main/config/configtls/README.md) on how to configure TLS. In this example, we enable TLS for receivers which leverages server configuration. 

Example
```
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: mysite.local:55690
        tls:
          cert_file: server.crt
          key_file: server.key
```

## Client Authentication
Envoy [sandbox for TLS](https://www.envoyproxy.io/docs/envoy/latest/start/sandboxes/tls) is a good source for reference. A few elements in following configuration are good to call out,
- `require_client_certificate` is set true
- The `matcher` under `validation_context` expects a client certificate with `group-id` as well as `tenat-id` which provides more granular control.

```
typed_config:
    "@type": type.googleapis.com/envoy.extensions.transport_sockets.tls.v3.DownstreamTlsContext
    require_client_certificate: true
    common_tls_context:
    # h2 If the listener is going to support both HTTP/2 and HTTP/1.1.
    alpn_protocols: "h2"
    tls_certificates:
    - certificate_chain: 
        filename: "/etc/envoy.crt"
        private_key: 
        filename: "/etc/envoy.key"
    validation_context:
        match_typed_subject_alt_names:
        - san_type: URI
        matcher:
            Match tenant by two level info: group id and tenant-id.
            exact: "aprn:trace-client:certmgr:::group-x:/ig/5003178/uv/tenant-a"
        trusted_ca:
        filename: "/etc/ca.crt"
```

## Setup Environment
### Generate Certificates
To generate various self-signed certificates, including those for Envoy and the OpenTelemetry Collector receiver, as well as tracing client certificate, we utilize the widely renowned open-source tool [OpenSSL](https://www.openssl.org/source/), OpenSSL 3.1.0 14 was tested. 

In the `certs` folder, you can find a set of `.ext` files which define the properties for a certificate. A `MakeFile` is provided to facilitate the process. 

```
$ cd certs
$ make clean && make all
```
### Bring up services
We use docker compose to bring up Envoy and OpenTelemetry Collection Pipeline. Make sure the current folder is `secure-tracing`, 

```
$ docker compose up
```
From the console window, verify that `Envoy` and `Collector` are up and running. If you see error similar to following,

```
secure-tracing-otel-collector-1  | Error: cannot start pipelines: failed to load TLS config: failed to load TLS cert and key: read /etc/otel-collector.crt: is a directory
```
It's most likely due to missing certificates. Follow the steps from section above to generate certificates. 

## Run test

### Compile telemetrygen
From the root of this repository, 
```
$ cd cmd/telemetrygen
$ go build . 
$ cp telemetrygen ../../examples/secure-tracing
```

### Send a trace
From the root of this repository,
```
$ cd examples/secure-tracing
$ chmod +x telemetrygen
$ ./telemetrygen traces --traces 1 --otlp-endpoint 127.0.0.1:10000 --ca-cert 'certs/ca.crt' --mtls --client-cert 'certs/tracing-client.crt' --client-key 'certs/tracing-client.key'
```

Verify traces are captured and sent to console running `docker compose up`, similar to following ...
```
secure-tracing-otel-collector-1  |      -> service.name: Str(telemetrygen)
secure-tracing-otel-collector-1  | ScopeSpans #0
secure-tracing-otel-collector-1  | ScopeSpans SchemaURL: 
secure-tracing-otel-collector-1  | InstrumentationScope telemetrygen 
secure-tracing-otel-collector-1  | Span #0
secure-tracing-otel-collector-1  |     Trace ID       : 0fe7ca900fda938ce918f8b2d82d6ae9
secure-tracing-otel-collector-1  |     Parent ID      : 1011b3f973049923
secure-tracing-otel-collector-1  |     ID             : 65f3cfe524375dd4
secure-tracing-otel-collector-1  |     Name           : okey-dokey
secure-tracing-otel-collector-1  |     Kind           : Server
...
secure-tracing-otel-collector-1  | Attributes:
secure-tracing-otel-collector-1  |      -> net.peer.ip: Str(1.2.3.4)
secure-tracing-otel-collector-1  |      -> peer.service: Str(telemetrygen-client)
...

``` 
## Shutdown Services
```
$ docker compose down
```

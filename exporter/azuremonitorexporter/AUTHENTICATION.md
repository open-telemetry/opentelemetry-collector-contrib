# Authentication

## Local Authentication

The default authentication mechanism used by the Azure Monitor Exporter is "Local Authentication" - based solely on the Application Insights Instrumentation Key. An example of the exporters section of the configuration is

```yaml
exporters:
   azuremonitor:
      instrumentation_key: 00000000-0000-0000-0000-000000000000
```

Where the zero guid is replaced with the instrumentation key of your Application Insights instance.

The same can be achieved by using an environment variable to hold the key

```yaml
exporters:
   azuremonitor:
      instrumentation_key: ${env:APP_INSIGHTS_KEY}
```

## AAD/Entra Authentication

Local Authentication can be disabled in [Applitiiton Insights](https://learn.microsoft.com/en-us/azure/azure-monitor/app/azure-ad-authentication) and an AAD based identity can be used in conjunction with the instrumentation key. The instrumentation key is still required by the Exporter, but it is no long  sufficient to just have this in order to import data.

The Azure Monitor Exporter does not support this approach directly, but it can be used with the [AAD Authentication Proxy](https://github.com/Azure/aad-auth-proxy) from the Azure Monitor product group.

The AAD Auth Proxy is a separate container/side-car that proxies calls to the Application Insights ingestion endpoint and attaches a bearer token to each call, asserting an AAD identity. This identity is managed by a certificate in the container that is registered with a Service Principal in AAD.

To use this, both Azure Monitor Exporter and the AAD Auth Proxy require specific configuration. In the Exporter's configuration, it is necessary to override the default endpoint used to send data to. In the following example, it is assumed the AAD Auth Proxy is listening on localhost:8081.

```yaml
exporters:
   azuremonitor:
      instrumentation_key: ${env:APP_INSIGHTS_KEY}
      endpoint: http://localhost:8081/v2.1/track
```

The key difference is this uses the `v2.1` endpoint, not the standard `v2` one.

In the docker compose file for AAD Auth Proxy, the following values need to be set:

```docker
azuremonitor-ingestion-proxy:
    image: mcr.microsoft.com/azuremonitor/auth-proxy/prod/aad-auth-proxy/images/aad-auth-proxy:{latest version}
    restart: always
    volumes:
      - ./certs:/certs
    ports:
      - "8081:8081"
    environment:
      AUDIENCE: "https://monitor.azure.com/.default"
      TARGET_HOST: "{application insights ingestion endpoint}"
      LISTENING_PORT: "8081"
      IDENTITY_TYPE: "aadApplication"
      AAD_CLIENT_ID: "{service principal client id}"
      AAD_TENANT_ID: "{service principal tenant id}"
      AAD_CLIENT_CERTIFICATE_PATH: "{path to certificate}"
```

- `AUDIENCE`: value is the generic Azure Monitor Scope.
- `TARGET_HOST`: the Application Insights IngestionEndpoint value from the Connection String, available in the Azure Portal.
- `AAD_CLIENT_ID`: client id of the service principal representing the AAD identity to use.
- `AAD_TENANT_ID`: id of the AAD Tenant the service principal exists in.
- `AAD_CLIENT_CERTIFICATE_PATH`: path to the .pem certificate file containing the CERTIFICATE and PRIVATE KEY parts of the certificate registered with the service principal.



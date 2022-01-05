# ASAP Client Authentication Extension

This extension provides [Atlassian Service Authentication Protocol](https://s2sauth.bitbucket.io/) (ASAP) client 
credentials for HTTP or gRPC based exporters. 

## Example Configuration

```yaml
extensions:
  asapclient:
    # The `kid` as specified by the asap specification.
    key_id: somekeyid
    # The `iss` as specified by the asap specification.
    issuer: someissuer
    # The `aud` as specified by the asap specification.
    audience:
      - someservice
      - someotherservice
    # The private key of the client, used to sign the token. For an example, see `testdata/config.yaml`.
    private_key: ${ASAP_PRIVATE_KEY}
    # The time until expiry of each given token. The token will be cached and then re-provisioned upon expiry. 
    # For more info see the "exp" claim in the asap specification: https://s2sauth.bitbucket.io/spec/#access-token-generation
    ttl: 60s
    
exporters:
  otlphttp/withauth:
    endpoint: http://localhost:9000
    auth:
      authenticator: asapclient

  otlp/withauth:
    endpoint: 0.0.0.0:5000
    ca_file: /tmp/certs/ca.pem
    auth:
      authenticator: asapclient    
```

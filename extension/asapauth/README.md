# ASAP Client Authentication Extension

This extension provides [Atlassian Service Authentication Protocol](https://s2sauth.bitbucket.io/) (ASAP) client 
credentials for HTTP or RPC based exporters. 

## Example Configuration

```yaml
extensions:
  asapclient:
    key_id: somekeyid
    issuer: someissuer
    audience:
      - someservice
      - someotherservice
    private_key: ${ASAP_PRIVATE_KEY}
    ttl_seconds: 120 # Default: 60
    
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

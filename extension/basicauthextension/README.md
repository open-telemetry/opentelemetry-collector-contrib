# Basic Authenticator

This extension implements both `configauth.ServerAuthenticator` and `configauth.ClientAuthenticator` to authenticate clients and servers using HTTP Basic Authentication. The authenticator type has to be set to `basicauth`.

When used as ServerAuthenticator, if the authentication is successful `client.Info.Auth` will expose the following attributes:

- `username`: The username of the authenticated user.
- `raw`: Raw base64 encoded credentials.

The configuration should specify only one instance of `basicauth` extension for either client or server authentication. 

The following are the configuration options:

- `htpasswd.file`:  The path to the htpasswd file.
- `htpasswd.inline`: The htpasswd file inline content.
- `client_auth`: Single username password combination in the form of `username:password` for client authentication.

To configure the extension as a server authenticator, either one of `htpasswd.file` or `htpasswd.inline` has to be set. If both are configured, `htpasswd.inline` credentials take precedence.

To configure the extension as a client authenticator, `client_auth` has to be set.

If both the options are configured, the extension will throw an error.
## Configuration

```yaml
extensions:
  basicauth/server:
    htpasswd: 
      file: .htpasswd
      inline: |
        ${BASIC_AUTH_USERNAME}:${BASIC_AUTH_PASSWORD}
  
  basicauth/client:
    client_auth: username:password

receivers:
  otlp:
    protocols:
      http:
        auth:
          authenticator: basicauth/server

processors:

exporters:
  otlphttp:
    auth:
      authenticator: basicauth/client 

service:
  extensions: [basicauth/server, basicauth/client]
  pipelines:
    traces:
      receivers: [otlp]
      processors: []
      exporters: [otlphttp]
```

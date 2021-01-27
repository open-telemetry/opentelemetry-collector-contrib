# Connecting through a proxy

Stanza supports sending logs through a HTTP proxy. To enable this, set the environment variables HTTP_PROXY and HTTPS_PROXY to the address of your proxy server.

For example:

```bash
export HTTP_PROXY=http://user:password@myproxy:3128
export HTTPS_PROXY=http://user:password@myproxy:3128
stanza -c ./config.yaml
```

To set this for the Stanza service on Linux, the service file can be modified with `systemctl edit --full stanza`, and add the following lines in the `[Service]` section.

```service
Environment=HTTP_PROXY=http://user:password@myproxy:3128
Environment=HTTPS_PROXY=http://user:password@myproxy:3128
```

## Using a self-signed certificate

If your proxy uses a self-signed certificate, it will need to be installed in the OS's trusted certificate store. For an example of how to do that on RedHat Linux, see [here](https://www.redhat.com/sysadmin/ca-certificates-cli) under the heading "Updating ca-certificates to validate sites with an internal CA certificate".

## Known issues

`go-grpc`, which is used for the Google Cloud Output operator, does not currently support sending a proxy `CONNECT` call over HTTPS, so proxy URLs must start with `http://`. Once the `CONNECT` request is made, a TLS tunnel will still be established, encrypting all the logs that are sent.

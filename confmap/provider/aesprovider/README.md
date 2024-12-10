## Summary

This package provides a `confmap.Provider` implementation for symmetric AES encryption of credentials (and other sensitive values) in configurations. It relies on the environment variable `OTEL_AES_CREDENTIAL_PROVIDER` set to the value of the AES key, base64 encoded. 16, 24, or 32 byte keys are supported, selecting AES-128, AES-192, or AES-256 respectively.

An AES 32-byte (AES-256) key can be generated using the following command:

```shell
openssl rand -base64 32
```

## How it works
 Use placeholders with the following pattern `${aes:<encrypted & base64-encoded value>}` in a configuration. The value will be decrypted using the AES key provided in the environment variable `OTEL_AES_CREDENTIAL_PROVIDER`

> For example:
> 
> ```shell
> export OTEL_AES_CREDENTIAL_PROVIDER="GQi+Y8HwOYzs8lAOjHUqB7vXlN8bVU2k0TAKtzwJzac="
> ```
> 
> ```yaml
> password: ${aes:RsEf6cTWrssi8tlssfs1AJs2bRMrVm2Ce5TaWPY=}
> ```
> 
> will resolve to:
> ```yaml
> password: '1'
> ```

## Caveats

Since AES is a symmetric encryption algorithm, the same key must be used to encrypt and decrypt the values. If the key needs to be exchanged between the collector and a server, it should be done over a secure connection.

When the collector persists its configuration to disk, storing the key in the environment prevents compromising secrets in the configuration. It still presents a vulnerability if the attacker has access to the collector's memory or the environment's configuration, but increases security over plaintext configurations.
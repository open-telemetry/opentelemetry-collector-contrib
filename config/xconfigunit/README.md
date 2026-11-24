# xconfigunit

This package provides `ByteSize`, an `int64` byte count that can be read from
and written to text configuration values. It implements `encoding.TextUnmarshaler`
and `encoding.TextMarshaler`, so it can be used in Collector configuration structs.

Values may combine byte units, for example `100MB20KB`. The units are decimal:
`KB` is 1,000 bytes, `MB` is 1,000,000 bytes, and `GB` is 1,000,000,000 bytes.
A leading minus sign applies to the whole value.

```yaml
max_size: 100MB20KB
```

After decoding, convert `ByteSize` to `int64` to get the byte count:

```go
byteCount := int64(config.MaxSize)
```

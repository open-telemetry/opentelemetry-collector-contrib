# ByteSize

ByteSizes are a type that allows specifying a number of bytes in a human-readable format. See the examples for details.


## Examples

### Various ways to specify 5000 bytes

```yaml
- type: some_operator
  bytes: 5000
```

```yaml
- type: some_operator
  bytes: 5kb
```

```yaml
- type: some_operator
  bytes: 4.88KiB
```

```yaml
- type: some_operator
  bytes: 5e3
```

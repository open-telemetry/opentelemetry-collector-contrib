# OTTL Profiles

This module houses OTTL's profiles support: the `ottlprofile` and
`ottlprofilesample` contexts, their profile-only internals, and the `ProfileID`
converter.

Profiles support is under active development and is versioned independently at
0.x so that it sits outside of OTTL's stability guarantees.

## Converters

### ProfileID

`ProfileID(bytes|string)`

The `ProfileID` Converter returns a `pprofile.ProfileID` struct from the given byte slice OR hex string.

`bytes`  byte slice of exactly 16 bytes.
`string` is a string of exactly 32 hex characters solely composed of valid hexadecimal chars.

Examples:

- `ProfileID(0x00112233445566778899aabbccddeeff)`
- `ProfileID("a389023abaa839283293ed323892389d")`

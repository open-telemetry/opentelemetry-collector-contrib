# Storage

A storage extension persist state beyond the collector process. Other components can request a storage client from the storage extension and use it to manage state. 

The `storage.Extension` interface contains the following methods:
```
Get(string) ([]byte, error) // returns error if not found
Set(string, []byte) error   // returns error if not set
Delete(string) error        // returns error if not found
```
Note: All methods should return error only if a problem occurred. (For example, if a fiel )

# TODO Sample code
- Document component expections

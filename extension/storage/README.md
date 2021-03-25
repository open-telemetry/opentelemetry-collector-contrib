# Storage

Storage is a singleton extension that can be used to persist state to disk. Other components can request a persistance interface from the storage extension, and use it to manage a persistent key/value store. 

The `Storage` interface contains the following methods:
```
Get(string) ([]byte, error) // returns error if not found
Set(string, []byte) error   // returns error if not set
Delete(string) error        // returns error if not found
```

# TODO

- Document configuration
- Sample code to get the extension, and to get the inteface from it
- Document component expections

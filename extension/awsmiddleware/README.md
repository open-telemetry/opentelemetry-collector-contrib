# AWS Middleware

An AWS middleware extension provides request and/or response handlers that can be configured on AWS SDK v1/v2 clients.
Other components can configure their AWS SDK clients using the `awsmiddleware.ConfigureSDKv1` and `awsmiddleware.ConfigureSDKv2` functions.

The `awsmiddleware.Extension` interface extends `component.Extension` by adding the following methods:
```
RequestHandlers() []RequestHandler
ResponseHandlers() []ResponseHandler
```

The `awsmiddleware.RequestHandler` interface contains the following methods:
```
ID() string
Position() HandlerPosition
HandleRequest(r *http.Request)
```

The `awsmiddleware.ResponseHandler` interface contains the following methods:
```
ID() string
Position() HandlerPosition
HandleResponse(r *http.Response)
```

- `ID` uniquely identifies a handler. Middleware will fail if there is clashing 
- `Position` determines whether the handler is appended to the front or back of the existing list. Insertion is done
in the order of the handlers provided.
- `HandleRequest/Response` provides a hook to handle the request/response before and after they've been sent.
# AWS Middleware

An AWS middleware extension provides request and/or response handlers that can be configured on AWS SDK v1/v2 clients.
Other components can configure their AWS SDK clients using `awsmiddleware.GetConfigurer` and passing the `SDKv1` or `SDKv2`
options into the `Configure` function available on the `Configurer`.

The `awsmiddleware.Extension` interface extends `component.Extension` by adding the following method:
```
Handlers() ([]RequestHandler, []ResponseHandler)
```

The `awsmiddleware.RequestHandler` interface contains the following methods:
```
ID() string
Position() HandlerPosition
HandleRequest(ctx context.Context, r *http.Request)
```

The `awsmiddleware.ResponseHandler` interface contains the following methods:
```
ID() string
Position() HandlerPosition
HandleResponse(ctx context.Context, r *http.Response)
```

- `ID` uniquely identifies a handler. Middleware will fail if there is clashing 
- `Position` determines whether the handler is appended to the front or back of the existing list. Insertion is done
in the order of the handlers provided.
- `HandleRequest/Response` provides a hook to handle the request/response before and after they've been sent along
with the context.

There are a functions available that can be used to extract metadata from the context.

# extension/opampcustommessages

## Overview

This modules contains interfaces and shared code for sending and receiving [custom messages](https://github.com/open-telemetry/opamp-spec/blob/main/specification.md#custom-messages) via OpAMP.

## Usage

An extension may implement the `opampcustommessages.CustomCapabilityRegistry` interface, which allows other components to register capabilities to send and receive messages to/from an OpAMP server. For an example of a component implementing this interface, see the [OpAMP extension](../opampextension/README.md).


### Registering a custom capability

Other components may use a configured OpAMP extension to send and receive custom messages to and from an OpAMP server. Components may use the provided `components.Host` from the Start method in order to get a handle to the registry:

```go
func Start(_ context.Context, host component.Host) error {
	ext, ok := host.GetExtensions()[opampExtensionID]
	if !ok {
		return fmt.Errorf("extension %q does not exist", opampExtensionID)
	}

	registry, ok := ext.(opampcustommessages.CustomCapabilityRegistry)
	if !ok {
		return fmt.Errorf("extension %q is not a custom message registry", opampExtensionID)
	}

	handler, err := registry.Register("io.opentelemetry.custom-capability")
	if err != nil {
		return fmt.Errorf("failed to register custom capability: %w", err)
	}

	// ... send/receive messages using the given handler

	return nil
}
```

### Using a CustomCapabilityHandler to send/receive messages

After obtaining a handler for the custom capability, you can send and receive messages for the custom capability by using the SendMessage and Message methods, respectively:

#### Sending a message

To send a message, you can use the SendMessage method. Since only one custom message can be scheduled to send at a time, the error returned should be checked if it's [ErrCustomMessagePending](https://pkg.go.dev/github.com/open-telemetry/opamp-go@v0.14.0/client/types#pkg-variables), and wait on the returned channel to attempt sending the message again.

```go
for {
	sendingChan, err := handler.SendMessage("messageType", []byte("message-data"))
	switch {
	case err == nil:
		break
	case errors.Is(err, types.ErrCustomMessagePending):
		<-sendingChan
		continue
	default:
		return fmt.Errorf("failed to send message: %w", err)
	}
}
```

#### Receiving a message

Messages can be received through the channel returned by the `Message` method on the handler:

```go
msg := <-handler.Message()
// process the message...
```

Components receiving messages should take care not to modify the received message, as the message may be shared between multiple components.

### Unregistering a capability

After a component is done processing messages for a given capability, or shuts down, it should unregister its handler. You can do this by calling the `Unregister` method:

```go
handler.Unregister()
```

After a handler has been unregistered, it will no longer receive any messages from the OpAMP server, and any further calls to SendMessage will reject the message and return an error.

package sporadicconnector

import (
	"errors"
	"math/rand"

	"go.opentelemetry.io/collector/consumer/consumererror"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var errNonPermanent = errors.New("non permanent error")
var errPermanent = errors.New("permanent error")

// randomNonPermanentError is a decision function that succeeds approximately
// half of the time and fails with a non-permanent error the rest of the time.
func randomNonPermanentError() error {
	code := codes.Unavailable
	s := status.New(code, errNonPermanent.Error())
	if rand.Float32() < 0.5 {
		return s.Err()
	}
	return nil
}

// randomPermanentError is a decision function that succeeds approximately
// half of the time and fails with a permanent error the rest of the time.
func randomPermanentError() error {
	if rand.Float32() < 0.5 {
		return consumererror.NewPermanent(errPermanent)
	}
	return nil
}

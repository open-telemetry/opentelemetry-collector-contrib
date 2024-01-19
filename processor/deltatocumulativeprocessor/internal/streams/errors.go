package streams

import (
	"fmt"
)

func Error(id Ident, err error) error {
	return StreamErr{Ident: id, Err: err}
}

type StreamErr struct {
	Ident Ident
	Err   error
}

func (e StreamErr) Error() string {
	return fmt.Sprintf("%s: %s", e.Ident, e.Err)
}

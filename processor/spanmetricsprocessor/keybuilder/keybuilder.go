package keybuilder

import "strings"

const (
	separator = string(byte(0))
	defaultCapacity = 1024
)

type KeyBuilder interface {
	Append(value ...string)
	String() string
}

type keyBuilder struct {
	sb        strings.Builder
	separator string
}

func New() KeyBuilder {
	b := keyBuilder{
		sb:        strings.Builder{},
		separator: separator,
	}
	b.sb.Grow(defaultCapacity)
	return &b
}

func (mkb *keyBuilder) Append(values ...string) {
	for _, value := range values {
		if len(value) == 0 {
			continue
		}
		if mkb.sb.Len() != 0 {
			mkb.sb.WriteString(mkb.separator)
		}
		// It's worth noting that from pprof benchmarks, WriteString is the most expensive operation of this processor.
		// Specifically, the need to grow the underlying []byte slice to make room for the appended string.
		mkb.sb.WriteString(value)
	}
}

func (mkb *keyBuilder) String() string {
	return mkb.sb.String()
}

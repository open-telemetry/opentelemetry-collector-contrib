package fileconsumer

import (
	"bufio"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

type splitterFactory struct {
	EncodingConfig helper.EncodingConfig
	Flusher        helper.FlusherConfig
	SplitFunc      bufio.SplitFunc
}

func (factory *splitterFactory) Build() (*helper.Splitter, error) {
	enc, err := factory.EncodingConfig.Build()
	if err != nil {
		return nil, err
	}
	flusher := factory.Flusher.Build()
	splitFunc := factory.SplitFunc
	if flusher != nil {
		splitFunc = flusher.SplitFunc(splitFunc)
	}
	return &helper.Splitter{
		Encoding:  enc,
		Flusher:   flusher,
		SplitFunc: splitFunc,
	}, nil
}

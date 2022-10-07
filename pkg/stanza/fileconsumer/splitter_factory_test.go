package fileconsumer

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

func Test_multilineSplitterFactory_Build(t *testing.T) {
	type fields struct {
		EncodingConfig helper.EncodingConfig
		Flusher        helper.FlusherConfig
		Multiline      helper.MultilineConfig
	}
	type args struct {
		maxLogSize int
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "default configuration",
			fields: fields{
				EncodingConfig: helper.NewEncodingConfig(),
				Flusher:        helper.NewFlusherConfig(),
				Multiline:      helper.NewMultilineConfig(),
			},
			args: args{
				maxLogSize: defaultMaxLogSize,
			},
			wantErr: false,
		},
		{
			name: "eoncoding error",
			fields: fields{
				EncodingConfig: helper.EncodingConfig{
					Encoding: "error",
				},
				Flusher:   helper.NewFlusherConfig(),
				Multiline: helper.NewMultilineConfig(),
			},
			args: args{
				maxLogSize: defaultMaxLogSize,
			},
			wantErr: true,
		},
		{
			name: "Multiline  error",
			fields: fields{
				EncodingConfig: helper.NewEncodingConfig(),
				Flusher:        helper.NewFlusherConfig(),
				Multiline: helper.MultilineConfig{
					LineStartPattern: "START",
					LineEndPattern:   "END",
				},
			},
			args: args{
				maxLogSize: defaultMaxLogSize,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := &multilineSplitterFactory{
				EncodingConfig: tt.fields.EncodingConfig,
				Flusher:        tt.fields.Flusher,
				Multiline:      tt.fields.Multiline,
			}
			got, err := factory.Build(tt.args.maxLogSize)
			if (err != nil) != tt.wantErr {
				t.Errorf("Build() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				assert.NotNil(t, got)
			}
		})
	}
}

func Test_newMultilineSplitterFactory(t *testing.T) {
	splitter := newMultilineSplitterFactory(helper.NewEncodingConfig(), helper.NewFlusherConfig(), helper.NewMultilineConfig())
	assert.NotNil(t, splitter)
}

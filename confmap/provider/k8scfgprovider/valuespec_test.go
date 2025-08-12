// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8scfgprovider

import (
	"reflect"
	"testing"
)

func Test_parseURI(t *testing.T) {
	type args struct {
		uri string
	}
	tests := []struct {
		name    string
		args    args
		want    *valueSpec
		wantErr bool
	}{
		{
			name: "valid secret uri",
			args: args{
				uri: "k8scfg:secret/ns1/secret1/data/key",
			},
			want: &valueSpec{
				kind:      "secret",
				namespace: "ns1",
				name:      "secret1",
				dataType:  "data",
				key:       "key",
			},
			wantErr: false,
		},
		{
			name: "valid secret uri with dot namespace",
			args: args{
				uri: "k8scfg:secret/./secret1/data/key",
			},
			want: &valueSpec{
				kind:      "secret",
				namespace: ".",
				name:      "secret1",
				dataType:  "data",
				key:       "key",
			},
			wantErr: false,
		},
		{
			name: "invalid dataType secret",
			args: args{
				uri: "k8scfg:secret/./secret1/binaryData/key",
			},
			wantErr: true,
		},
		{
			name: "invalid dataType configMap",
			args: args{
				uri: "k8scfg:configMap/./secret1/stringData/key",
			},
			wantErr: true,
		},
		{
			name: "not opaque uri",
			args: args{
				uri: "k8scfg://secret/ns1/secret1/data/key",
			},
			wantErr: true,
		},
		{
			name: "bad scheme",
			args: args{
				uri: "k8scfg1://secret/ns1/secret1/data/key",
			},
			wantErr: true,
		},
		{
			name: "too few parts",
			args: args{
				uri: "k8scfg:secret/ns1/secret1/data",
			},
			wantErr: true,
		},
		{
			name: "too many parts",
			args: args{
				uri: "k8scfg:secret/ns1/secret1/data/key/tomany",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseURI(tt.args.uri)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseURI() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseURI() = %v, want %v", got, tt.want)
			}
		})
	}
}

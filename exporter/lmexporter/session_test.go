package lmexporter

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func Test_lmv1Account_name(t *testing.T) {
	type fields struct {
		accountName string
		accessID    string
		accessKey   string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		// TODO: Add test cases.
		{
			"Blank arguments test",
			fields{
				"",
				"",
				"",
			},
			"",
		},
		{
			"Sample test case-1",
			fields{
				"localdev",
				"1",
				"NDhOQ1R0ck5MXl9CcjZpNThhezZ1ZmNJW1s0Q2d+ZUZjfXYtbjN1bn1GM0ppSVZ+ayVKXng1bTRmMjRkNms3KA==",
			},
			"localdev",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := lmv1Account{
				accountName: tt.fields.accountName,
				accessID:    tt.fields.accessID,
				accessKey:   tt.fields.accessKey,
			}
			if got := a.name(); got != tt.want {
				t.Errorf("lmv1Account.name() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_lmv1Account_id(t *testing.T) {
	type fields struct {
		accountName string
		accessID    string
		accessKey   string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		// TODO: Add test cases.
		{
			"Blank arguments test",
			fields{
				"",
				"",
				"",
			},
			"",
		},
		{
			"Sample test case-1",
			fields{
				"localdev",
				"1",
				"NDhOQ1R0ck5MXl9CcjZpNThhezZ1ZmNJW1s0Q2d+ZUZjfXYtbjN1bn1GM0ppSVZ+ayVKXng1bTRmMjRkNms3KA==",
			},
			"1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := lmv1Account{
				accountName: tt.fields.accountName,
				accessID:    tt.fields.accessID,
				accessKey:   tt.fields.accessKey,
			}
			if got := a.id(); got != tt.want {
				t.Errorf("lmv1Account.id() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_lmv1Account_token(t *testing.T) {
	type fields struct {
		accountName string
		accessID    string
		accessKey   string
	}
	type args struct {
		method string
		data   []byte
		uri    string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			"Sample test case-1",
			fields{
				"localdev",
				"MButz9r3iiz458G354nh",
				"Up83q}-6eCLGH95nU6R2)7EJIAE%bahE)L5G7qf9",
			},
			args{
				http.MethodGet,
				[]byte(""),
				"https://localdev.logicmonitor.com/santaba/rest/ping",
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := lmv1Account{
				accountName: tt.fields.accountName,
				accessID:    tt.fields.accessID,
				accessKey:   tt.fields.accessKey,
			}
			token := a.token(tt.args.method, tt.args.data, tt.args.uri)

			res := strings.Split(token, ":")
			if res[0] != "LMv1 "+tt.fields.accessID {
				t.Error("a.token() generates wrong LMv1 token")
			}
		})
	}
}

func Test_bearerAccount_name(t *testing.T) {
	type fields struct {
		accountName string
		key         string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		// TODO: Add test cases.
		{
			"Blank arguments test",
			fields{
				"",
				"",
			},
			"",
		},
		{
			"Sample test case-1",
			fields{
				"localdev",
				"Q2s4NDl6S3gzcjdaZmpZdEM5bks6M0V6UEtMZG5zT1p4aWNpc0Z5YnVTdz09",
			},
			"localdev",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := bearerAccount{
				accountName: tt.fields.accountName,
				key:         tt.fields.key,
			}
			if got := b.name(); got != tt.want {
				t.Errorf("bearerAccount.name() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_bearerAccount_id(t *testing.T) {
	type fields struct {
		accountName string
		key         string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		// TODO: Add test cases.
		{
			"Blank arguments test",
			fields{
				"",
				"",
			},
			"",
		},
		{
			"Sample test case-1",
			fields{
				"localdev",
				"g96qSJx5E83P84xNcfUa:(+CH7s3R[Q4a4_EDyFv3N74=t)Eu^9G_8{J5UcR^",
			},
			"g96qSJx5E83P84xNcfUa",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := bearerAccount{
				accountName: tt.fields.accountName,
				key:         tt.fields.key,
			}
			if got := b.id(); got != tt.want {
				t.Errorf("bearerAccount.id() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_bearerAccount_token(t *testing.T) {
	type fields struct {
		accountName string
		key         string
	}
	type args struct {
		method string
		data   []byte
		uri    string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   string
	}{
		// TODO: Add test cases.
		{
			"Blank arguments test",
			fields{
				"",
				"",
			},
			args{
				"",
				[]byte(""),
				"",
			},
			"Bearer ",
		},
		{
			"Sample test case-1",
			fields{
				"localdev",
				"g96qSJx5E83P84xNcfUa:(+CH7s3R[Q4a4_EDyFv3N74=t)Eu^9G_8{J5UcR^",
			},
			args{
				"",
				[]byte(""),
				"",
			},
			"Bearer g96qSJx5E83P84xNcfUa:(+CH7s3R[Q4a4_EDyFv3N74=t)Eu^9G_8{J5UcR^",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := bearerAccount{
				accountName: tt.fields.accountName,
				key:         tt.fields.key,
			}
			if got := b.token(tt.args.method, tt.args.data, tt.args.uri); got != tt.want {
				t.Errorf("bearerAccount.token() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewLMHTTPClient_APItoken(t *testing.T) {

	os.Setenv("LOGICMONITOR_ACCOUNT", "localdev")
	apitoken := make(map[string]string)
	apitoken["access_id"] = "GvGs48z52b25L648C3s8"
	apitoken["access_key"] = "~wfe=5E)Y844[xj}h=xCBPAn]{9mb}3mk_nd4[n["

	if got := NewLMHTTPClient(apitoken, nil, true); got == nil {
		t.Errorf("Got NewLMHTTPClient() = %v , want `not nil`", got)
	}
}

func TestNewLMHTTPClient_BearerToken(t *testing.T) {

	os.Setenv("LOGICMONITOR_ACCOUNT", "localdev")
	headers := make(map[string]string)
	headers["authorization"] = "Bearer UkpnODZhdWs0V0JZd1V4ODUzUXk6azdRbzFJOHBFWFJSVDBJYXNOamVmUT09"
	if got := NewLMHTTPClient(nil, headers, true); got == nil {
		t.Errorf("Got NewLMHTTPClient() = %v , want `not nil`", got)
	}
}

// func TestNewLMHTTPClient_APItoken_ENV(t *testing.T) {

// 	os.Setenv("LOGICMONITOR_ACCESS_ID", "GvGs48z52b25L648C3s8")
// 	os.Setenv("LOGICMONITOR_ACCESS_KEY", "~wfe=5E)Y844[xj}h=xCBPAn]{9mb}3mk_nd4[n[")
// 	if got := NewLMHTTPClient(nil, nil, true); got == nil {
// 		t.Errorf("Got NewLMHTTPClient() = %v , want `not nil`", got)
// 	}
// }

// func TestNewLMHTTPClient_Bearertoken_ENV(t *testing.T) {

// 	os.Unsetenv("LOGICMONITOR_ACCESS_ID")
// 	os.Unsetenv("LOGICMONITOR_ACCESS_KEY")
// 	os.Setenv("LOGICMONITOR_BEARER_TOKEN", "Bearer UkpnODZhdWs0V0JZd1V4ODUzUXk6azdRbzFJOHBFWFJSVDBJYXNOamVmUT09")
// 	if got := NewLMHTTPClient(nil, nil, true); got == nil {
// 		t.Errorf("Got NewLMHTTPClient() = %v , want `not nil`", got)
// 	}
// }

func TestLMhttpClient_MakeRequest(t *testing.T) {

	m := http.NewServeMux()
	m.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<?xml version="1.0" encoding="UTF-8" ?><feed  version="1.0" hasPendingRequests="false" ><company></company><status>200</status><errmsg>OK</errmsg><interval>1634019117032</interval></feed>`))
	})
	ts := httptest.NewServer(m)

	a := lmv1Account{
		accountName: "localdev",
		accessID:    "GvGs48z52b25L648C3s8",
		accessKey:   "~wfe=5E)Y844[xj}h=xCBPAn]{9mb}3mk_nd4[n[",
	}

	type args struct {
		version   string
		method    string
		baseURI   string
		uri       string
		configURL string
		timeout   time.Duration
		pBytes    *bytes.Buffer
		headers   map[string]string
	}
	tests := []struct {
		name           string
		mockHttpClient LMhttpClient
		args           args
		wantErr        bool
	}{
		{
			"Make Request: http.MethodGet",
			LMhttpClient{
				client: ts.Client(),
			},
			args{
				"3",
				http.MethodGet,
				"",
				"",
				ts.URL,
				5 * time.Second,
				nil,
				nil,
			},
			false,
		},
		{
			"Make Request : http.MethodPost",
			LMhttpClient{
				client: ts.Client(),
				aInfo:  a,
			},
			args{
				"3",
				http.MethodPost,
				"",
				"",
				ts.URL,
				5 * time.Second,
				bytes.NewBuffer([]byte("body")),
				nil,
			},
			false,
		},
		{
			"Make Request : Error",
			LMhttpClient{
				client: ts.Client(),
			},
			args{
				"3",
				http.MethodPost,
				"/santaba/api",
				"/ping",
				"",
				5 * time.Second,
				bytes.NewBuffer([]byte("body")),
				nil,
			},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			_, err := tt.mockHttpClient.MakeRequest(tt.args.version, tt.args.method, tt.args.baseURI, tt.args.uri, tt.args.configURL, tt.args.timeout, tt.args.pBytes, tt.args.headers)
			if (err != nil) != tt.wantErr {
				t.Errorf("LMhttpClient.MakeRequest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestLMhttpClient_GetContent(t *testing.T) {

	type args struct {
		url string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases
		{
			"Get content",
			args{
				"http://google.com",
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hc := NewLMHTTPClient(nil, nil, true)
			_, err := hc.GetContent(tt.args.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("LMhttpClient.GetContent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

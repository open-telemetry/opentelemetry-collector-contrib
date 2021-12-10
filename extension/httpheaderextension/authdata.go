package httpheaderextension

import "go.opentelemetry.io/collector/client"

var _ client.AuthData = (*authData)(nil)

type authData struct {
	headers map[string][]string
}

func (a *authData) GetAttribute(name string) interface{} {
	v, ok := a.headers[name]
	if !ok {
		return nil
	}
	if len(v) == 0 {
		return ""
	}
	return v[len(v)-1] // return last
}

func (a *authData) GetAttributeNames() []string {
	list := make([]string, len(a.headers))
	i := 0
	for k := range a.headers {
		list[i] = k
		i++
	}
	return list
}

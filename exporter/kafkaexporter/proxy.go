package kafkaexporter

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"

	"golang.org/x/net/proxy"
)

type httpsDialer struct{}

// HTTPSDialer is a https proxy: one that makes network connections on tls.
var HttpsDialer = httpsDialer{}
var TlsConfig = &tls.Config{}

func (d httpsDialer) Dial(network, addr string) (net.Conn, error) {

	c, err := tls.Dial("tcp", addr, TlsConfig)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	return c, nil
}

// httpProxy is a HTTP/HTTPS connect proxy.
type httpProxy struct {
	host     string
	haveAuth bool
	username string
	password string
	forward  proxy.Dialer
}

func newHTTPProxy(uri *url.URL, forward proxy.Dialer) (proxy.Dialer, error) {
	s := new(httpProxy)
	s.host = uri.Host
	s.forward = forward
	if uri.User != nil {
		s.haveAuth = true
		s.username = uri.User.Username()
		s.password, _ = uri.User.Password()
	}

	return s, nil
}

func (s *httpProxy) Dial(network, addr string) (net.Conn, error) {

	// Dial and create the https client connection.
	c, err := s.forward.Dial("tcp", s.host)
	if err != nil {
		return nil, err
	}

	// HACK. http.ReadRequest also does this.
	reqURL, err := url.Parse("http://" + addr)
	if err != nil {
		c.Close()
		return nil, err
	}

	//  //url
	reqURL.Scheme = ""
	req, err := http.NewRequest(http.MethodConnect, reqURL.String(), nil)
	if err != nil {

		return nil, err
	}

	req.Close = false
	if s.haveAuth {
		req.SetBasicAuth(s.username, s.password)
	}

	err = req.Write(c)
	if err != nil {
		c.Close()
		return nil, err
	}

	resp, err := http.ReadResponse(bufio.NewReader(c), req)
	if err != nil {
		resp.Body.Close()
		c.Close()
		return nil, err
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		c.Close()
		return nil, fmt.Errorf("Connect server using proxy error, StatusCode [%d]", resp.StatusCode)
	}

	return c, nil
}

func init() {
	proxy.RegisterDialerType("http", newHTTPProxy)
	proxy.RegisterDialerType("https", newHTTPProxy)
}

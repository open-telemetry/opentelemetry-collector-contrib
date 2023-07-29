// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package solacereceiver

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"reflect"
	"runtime"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"github.com/Azure/go-amqp"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/config/configtls"
	"go.uber.org/zap"
)

const (
	testQueueName    = "q"
	testReceiverName = "rx"

	// the majority of the amqp responses can be generated with github.com/Azure/go-amqp/internal/mocks or by wirecapturing amqp requests/responses

	// Hard coded protocol header indicating AMQP protocol id 0 version 1.0.0
	amqpProtocolHeaderResponse = "AMQP\x00\x01\x00\x00"
	// open response indicating we are open for business, contains a container id and a few other properties
	amqpOpenResponse = "\x00\x00\x00\x27\x02\x00\x00\x00\x00\x53\x10\xd0\x00\x00\x00\x17\x00\x00\x00\x05\xa1" +
		"\x09\x63\x6f\x6e\x74\x61\x69\x6e\x65\x72\x40\x40\x40\x70\x00\x00\xea\x60"
	// session begin response indicating various properties
	amqpSessionBeginResponse = "\x00\x00\x00\x25\x02\x00\x00\x01\x00\x53\x11\xc0\x18\x05\x60\x00" +
		"\x00\x70\x00\x00\x00\x01\x70\x7f\xff\xff\xff\x70\x7f\xff\xff\xff" +
		"\x70\x00\x00\x02\x3f"

	// attach response for a link named 'rx' as well as associated properties
	amqpAttachResponse = "\x00\x00\x00\xa6\x02\x00\x00\x01\x00\x53\x12\xd0\x00\x00\x00\x96" +
		"\x00\x00\x00\x0b\xb1\x00\x00\x00\x02" + testReceiverName + "\x70\x00\x00\x00\x01" +
		"\x42\x50\x00\x50\x00\x00\x53\x28\xd0\x00\x00\x00\x69\x00\x00\x00" +
		"\x0b\xb1\x00\x00\x00\x01" + testQueueName + "\x52\x00\xa3\x0bsession-end\x43\x42\x40\x40\x40\x00\x53\x26\x45\xe0" +
		"\x3b\x03\xa3\x12amqp:released:list\x12amqp:released:list\x12amqp:released:list\xe0\x08\x01\xa3" +
		"\x05queue\x00\x53\x29\x45\x40\x42\x43\x80\x00\x00" +
		"\x00\x00\x00\x98\x9a\x80"
	amqpDetachResponse = "\x00\x00\x00\x1a\x02\x00\x00\x01\x00\x53\x16\xd0\x00\x00\x00\x0a" +
		"\x00\x00\x00\x02\x70\x00\x00\x00\x01\x41"
	amqpSessionCloseResponse = "\x00\x00\x00\x14\x02\x00\x00\x01\x00\x53\x17\xd0\x00\x00\x00\x04" +
		"\x00\x00\x00\x00"
	amqpConnectionCloseResponse = "\x00\x00\x00\x14\x02\x00\x00\x00\x00\x53\x18\xd0\x00\x00\x00\x04" +
		"\x00\x00\x00\x00"

	// amqpBadOpenResponse is an open response with 'bad' as the container ID resulting in a failure when decoding the response in a variety of scenarios
	amqpBadOpenResponse = "\x00\x00\x00\x21\x02\x00\x00\x00\x00\x53\x10\xd0\x00\x00\x00\x11\x00\x00\x00\x05\xa1\x03\x62\x61\x64\x40\x40\x40\x70\x00\x00\xea\x60"

	amqpHelloWorldMsg = "\x00\x00\x00\xa9\x02\x00\x00\x01\x00\x53\x14\xc0\x18\x06\x70\x00" +
		"\x00\x00\x01\x70\x00\x00\x00\x00\xa0\x08\x00\x00\x00\x00\x00\x00" +
		"\x00\x01\x43\x40\x42\x00\x53\x70\xc0\x01\x00\x00\x53\x72\xd1\x00" +
		"\x00\x00\x2c\x00\x00\x00\x04\xa3\x0e\x78\x2d\x6f\x70\x74\x2d\x6a" +
		"\x6d\x73\x2d\x64\x65\x73\x74\x51\x00\xa3\x12\x78\x2d\x6f\x70\x74" +
		"\x2d\x6a\x6d\x73\x2d\x6d\x73\x67\x2d\x74\x79\x70\x65\x51\x03\x00" +
		"\x53\x73\xd0\x00\x00\x00\x31\x00\x00\x00\x0a\x40\x40\xa1\x01\x71" +
		"\x40\x40\x40\xa3\x18\x61\x70\x70\x6c\x69\x63\x61\x74\x69\x6f\x6e" +
		"\x2f\x6f\x63\x74\x65\x74\x2d\x73\x74\x72\x65\x61\x6d\x40\x40\x83" +
		"\x00\x00\x01\x80\xd3\xec\x22\xcd\x00\x53\x75\xa0\x0c\x48\x65\x6c" +
		"\x6c\x6f\x20\x77\x6f\x72\x6c\x64\x21"
)

func TestNewAMQPMessagingServiceFactory(t *testing.T) {
	broker := "some-broker:1234"
	queue := "someQueue"
	maxUnacked := int32(100)
	logger := zap.NewNop()
	tests := []struct {
		name    string
		cfg     *Config
		want    *amqpMessagingService
		wantErr bool
	}{
		// bad auth error
		{
			name: "expecting authentication errors",
			cfg: &Config{ // no password
				Auth:       Authentication{PlainText: &SaslPlainTextConfig{Username: "set"}},
				TLS:        configtls.TLSClientSetting{Insecure: false, InsecureSkipVerify: false},
				Broker:     []string{broker},
				Queue:      queue,
				MaxUnacked: maxUnacked,
			},
			wantErr: true,
		},
		// bad tls error
		{
			name: "expecting tls errors",
			cfg: &Config{ // invalid to only provide a key file
				Auth:       Authentication{PlainText: &SaslPlainTextConfig{Username: "user", Password: "password"}},
				TLS:        configtls.TLSClientSetting{TLSSetting: configtls.TLSSetting{KeyFile: "someKeyFile"}, Insecure: false},
				Broker:     []string{broker},
				Queue:      queue,
				MaxUnacked: maxUnacked,
			},
			wantErr: true,
		},
		// yes tls success secure
		{
			name: "expecting success with TLS expecting an amqps connection",
			cfg: &Config{ // invalid to only provide a key file
				Auth:       Authentication{PlainText: &SaslPlainTextConfig{Username: "user", Password: "password"}},
				TLS:        configtls.TLSClientSetting{Insecure: false},
				Broker:     []string{broker},
				Queue:      queue,
				MaxUnacked: maxUnacked,
			},
			want: &amqpMessagingService{
				connectConfig: &amqpConnectConfig{
					addr:       "amqps://" + broker,
					saslConfig: amqp.SASLTypePlain("user", "password"),
					tlsConfig:  &tls.Config{},
				},
				receiverConfig: &amqpReceiverConfig{
					queue:       queue,
					maxUnacked:  maxUnacked,
					batchMaxAge: 1 * time.Second,
				},
				logger: logger,
			},
		},
		// no tls success plaintext
		{
			name: "expecting success without TLS expecting an amqp connection",
			cfg: &Config{ // invalid to only provide a key file
				Auth:       Authentication{PlainText: &SaslPlainTextConfig{Username: "user", Password: "password"}},
				TLS:        configtls.TLSClientSetting{Insecure: true},
				Broker:     []string{broker},
				Queue:      queue,
				MaxUnacked: maxUnacked,
			},
			want: &amqpMessagingService{
				connectConfig: &amqpConnectConfig{
					addr:       "amqp://" + broker,
					saslConfig: amqp.SASLTypePlain("user", "password"),
					tlsConfig:  nil,
				},
				receiverConfig: &amqpReceiverConfig{
					queue:       queue,
					maxUnacked:  maxUnacked,
					batchMaxAge: 1 * time.Second,
				},
				logger: logger,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.want != nil && tt.want.connectConfig.saslConfig != nil {
				connSASLPlain = func(username, password string) amqp.SASLType {
					connSASLPlain = amqp.SASLTypePlain
					return tt.want.connectConfig.saslConfig
				}
			}

			factory, err := newAMQPMessagingServiceFactory(tt.cfg, logger)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, factory)
			} else {
				assert.NoError(t, err)
				actual := factory().(*amqpMessagingService)
				// assert that want == actual, checking individual fields (due to function pointers can't use deep equal)
				assert.Equal(t, tt.want.connectConfig.addr, actual.connectConfig.addr)
				testFunctionEquality(t, tt.want.connectConfig.saslConfig, actual.connectConfig.saslConfig)
				testFunctionEquality(t, tt.want.connectConfig.tlsConfig, actual.connectConfig.tlsConfig)
				assert.Equal(t, tt.want.receiverConfig, actual.receiverConfig)
				assert.Equal(t, tt.want.logger, actual.logger)
			}
		})
	}
}

func TestAMQPDialFailure(t *testing.T) {
	const expectedAddr = "some-host:1234"
	var expectedErr = fmt.Errorf("some error")
	dialFunc = func(ctx context.Context, addr string, opts *amqp.ConnOptions) (*amqp.Conn, error) {
		defer func() { dialFunc = amqp.Dial }() // reset dialFunc
		assert.Equal(t, expectedAddr, addr)
		return nil, expectedErr
	}
	service := &amqpMessagingService{
		connectConfig: &amqpConnectConfig{
			addr:       expectedAddr,
			saslConfig: nil,
			tlsConfig:  nil,
		},
		receiverConfig: &amqpReceiverConfig{
			queue:      "q",
			maxUnacked: 10000,
		},
		logger: zap.NewNop(),
	}
	err := service.dial(context.Background())
	assert.Equal(t, expectedErr, err)
}

func TestAMQPDialConfigOptionsWithoutTLS(t *testing.T) {
	// try creating a service without a tls config calling dial expecting no tls config passed
	const expectedAddr = "some-host:1234"
	var expectedErr = fmt.Errorf("some error")
	expectedAuthConnOption := amqp.SASLTypeAnonymous()
	dialFunc = func(ctx context.Context, addr string, opts *amqp.ConnOptions) (*amqp.Conn, error) {
		defer func() { dialFunc = amqp.Dial }() // reset dialFunc
		assert.Equal(t, expectedAddr, addr)
		testFunctionEquality(t, expectedAuthConnOption, opts.SASLType)
		// error out for simplicity
		return nil, expectedErr
	}
	service := &amqpMessagingService{
		connectConfig: &amqpConnectConfig{
			addr:       expectedAddr,
			saslConfig: expectedAuthConnOption,
			tlsConfig:  nil,
		},
		receiverConfig: &amqpReceiverConfig{
			queue:      "q",
			maxUnacked: 10000,
		},
		logger: zap.NewNop(),
	}
	err := service.dial(context.Background())
	assert.Equal(t, expectedErr, err)
}

func TestAMQPDialConfigOptionsWithTLS(t *testing.T) {
	// try creating a service with a tls config calling dial
	const expectedAddr = "some-host:1234"
	var expectedErr = fmt.Errorf("some error")
	expectedAuthConnOption := amqp.SASLTypeAnonymous()
	expectedTLSConnOption := &tls.Config{
		InsecureSkipVerify: false,
	}
	dialFunc = func(ctx context.Context, addr string, opts *amqp.ConnOptions) (*amqp.Conn, error) {
		defer func() { dialFunc = amqp.Dial }() // reset dialFunc
		assert.Equal(t, expectedAddr, addr)
		testFunctionEquality(t, expectedAuthConnOption, opts.SASLType)
		testFunctionEquality(t, expectedTLSConnOption, opts.TLSConfig)
		// error out for simplicity
		return nil, expectedErr
	}
	service := &amqpMessagingService{
		connectConfig: &amqpConnectConfig{
			addr:       expectedAddr,
			saslConfig: expectedAuthConnOption,
			tlsConfig:  expectedTLSConnOption,
		},
		receiverConfig: &amqpReceiverConfig{
			queue:      "q",
			maxUnacked: 10000,
		},
		logger: zap.NewNop(),
	}
	err := service.dial(context.Background())
	assert.Equal(t, expectedErr, err)
}

func TestAMQPNewClientDialAndCloseSuccess(t *testing.T) {
	service, conn := startMockedService(t)
	closeMockedAMQPService(t, service, conn)
}

// validate that we still proceed with close if a context is cancelled and no receiver/session close messages are sent
func TestAMQPNewClientDialAndCloseCtxTimeoutFailure(t *testing.T) {
	service, conn := startMockedService(t)

	closed := false
	conn.setCloseHandler(func() error {
		closed = true
		return nil
	})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	go func() {
		<-ctx.Done()
		// allow all other ctx waiters to finish first
		time.Sleep(10 * time.Millisecond)
		// notify the data channel that it is closed
		conn.nextData <- []byte(amqpConnectionCloseResponse)
	}()
	service.close(ctx)
	// expect conn.Close to have been called
	assert.True(t, closed)
}

// validate that we still proceed with close and don't panic when connection close throws an error
func TestAMQPNewClientDialAndCloseConnFailure(t *testing.T) {
	service, conn := startMockedService(t)

	writeData := [][]byte{[]byte(amqpDetachResponse), []byte(amqpSessionCloseResponse), []byte(amqpConnectionCloseResponse)}
	mockWriteData(conn, writeData)

	closed := false
	conn.setCloseHandler(func() error {
		closed = true
		return fmt.Errorf("some error")
	})
	service.close(context.Background())
	// expect conn.Close to have been called
	assert.True(t, closed)
}

func TestAMQPReceiveMessage(t *testing.T) {
	service, conn := startMockedService(t)
	conn.nextData <- []byte(amqpHelloWorldMsg)
	msg, err := service.receiveMessage(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, msg.GetData(), []byte("Hello world!"))
	closeMockedAMQPService(t, service, conn)
}

func TestAMQPReceiveMessageError(t *testing.T) {
	service, conn := startMockedService(t)
	// assert we are propagating the errors up
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	_, err := service.receiveMessage(ctx)
	assert.Error(t, err)
	closeMockedAMQPService(t, service, conn)
}

func TestAMQPAcknowledgeMessage(t *testing.T) {
	service, conn := startMockedService(t)
	conn.nextData <- []byte(amqpHelloWorldMsg)
	msg, err := service.receiveMessage(context.Background())
	assert.NoError(t, err)
	writeCalled := make(chan struct{})
	// Expected accept from AMQP frame for first received message
	// "\x00\x00\x00\x1c\x02\x00\x00\x00\x00\x53\x15\xd0\x00\x00\x00\x0c\x00\x00\x00\x05\x41\x43\x40\x41\x00\x53\x24\x45"
	conn.setWriteHandler(func(b []byte) (n int, err error) {
		// assert that a disposition is written
		assert.Equal(t, byte(0x15), b[10])
		assert.Equal(t, byte(0x24), b[26]) // 0x24 at the 27th byte in this case means accept
		close(writeCalled)
		return len(b), nil
	})
	err = service.accept(context.Background(), msg)
	assert.NoError(t, err)
	assertChannelClosed(t, writeCalled)
	closeMockedAMQPService(t, service, conn)
}

func TestAMQPModifyMessage(t *testing.T) {
	service, conn := startMockedService(t)
	conn.nextData <- []byte(amqpHelloWorldMsg)
	msg, err := service.receiveMessage(context.Background())
	assert.NoError(t, err)
	writeCalled := make(chan struct{})
	// Expected modify from AMQP frame for first received message
	// "\x00\x00\x00\x1c\x02\x00\x00\x00\x00\x53\x15\xd0\x00\x00\x00\x0c\x00\x00\x00\x05\x41\x43\x40\x41\x00\x53\x25\x45"
	conn.setWriteHandler(func(b []byte) (n int, err error) {
		// assert that a disposition is written
		assert.Equal(t, byte(0x15), b[10])
		assert.Equal(t, byte(0x27), b[26]) // 0x27 at the 27th byte in this case means modify
		close(writeCalled)
		return len(b), nil
	})
	err = service.failed(context.Background(), msg)
	assert.NoError(t, err)
	select {
	case <-writeCalled:
	case <-time.After(10 * time.Millisecond):
		t.Error("timed out waiting for write to be called")
	}
	closeMockedAMQPService(t, service, conn)
}

func startMockedService(t *testing.T) (*amqpMessagingService, *connMock) {
	conn := &connMock{
		nextData: make(chan []byte, 100),
	}
	mockDialFunc(conn)
	// writeData contains the responses used as mock data for amqp lifecycle
	// in the order of protocol, open, session, link attach, link detach, session close and connection close.
	writeData := [][]byte{[]byte(amqpProtocolHeaderResponse), []byte(amqpOpenResponse), []byte(amqpSessionBeginResponse), []byte(amqpAttachResponse)}
	// we need a custom write handler allowing for waiting until a flow start is called
	flowStartCalled := make(chan struct{})
	mockWriteData(conn, writeData, func(sentData, receivedData []byte) {
		if len(sentData) > 10 && sentData[10] == 19 { // check if the type is 19 (flow)
			close(flowStartCalled)
		}
	})

	service := &amqpMessagingService{
		connectConfig:  &amqpConnectConfig{addr: "some-addr"},
		receiverConfig: &amqpReceiverConfig{queue: "q", maxUnacked: 10000, batchMaxAge: 10 * time.Millisecond},
		logger:         zap.NewNop(),
	}
	err := service.dial(context.Background())
	assert.NoError(t, err)

	select {
	case <-flowStartCalled: // success, we can proceed with close
	case <-time.After(100 * time.Millisecond):
		t.Error("timed out waiting for flow start frame")
	}
	return service, conn
}

func closeMockedAMQPService(t *testing.T, service *amqpMessagingService, conn *connMock) {
	writeData := [][]byte{[]byte(amqpDetachResponse), []byte(amqpSessionCloseResponse), []byte(amqpConnectionCloseResponse)}
	mockWriteData(conn, writeData)

	closed := false
	conn.setCloseHandler(func() error {
		closed = true
		return nil
	})
	service.close(context.Background())
	// expect conn.Close to have been called
	assert.True(t, closed)
}

func TestAMQPNewClientDialWithBadSessionResponseExpectingError(t *testing.T) {
	conn := &connMock{
		nextData: make(chan []byte, 100),
	}
	mockDialFunc(conn)
	mockWriteData(conn, [][]byte{ // successful protocol header, open response, but fail with invalid open response, finally close
		[]byte(amqpProtocolHeaderResponse), []byte(amqpOpenResponse), []byte(amqpBadOpenResponse), []byte(amqpConnectionCloseResponse),
	})

	service := &amqpMessagingService{
		connectConfig: &amqpConnectConfig{
			addr: "some-addr",
		},
		logger: zap.NewNop(),
	}

	err := service.dial(context.Background())
	assert.Error(t, err)
	assert.NotNil(t, service.client)
	assert.Nil(t, service.session)
	assert.Nil(t, service.receiver)
}

func TestAMQPNewClientDialWithBadAttachResponseExpectingError(t *testing.T) {
	conn := &connMock{
		nextData: make(chan []byte, 100),
	}
	mockDialFunc(conn)
	mockWriteData(conn, [][]byte{ // successful protocol header, open response, begin response but fail with invalid attach response, finally close
		[]byte(amqpProtocolHeaderResponse), []byte(amqpOpenResponse), []byte(amqpSessionBeginResponse), []byte(amqpBadOpenResponse), []byte(amqpConnectionCloseResponse),
	})

	service := &amqpMessagingService{
		connectConfig: &amqpConnectConfig{
			addr: "some-addr",
		},
		receiverConfig: &amqpReceiverConfig{
			queue:      "q",
			maxUnacked: 10000,
		},
		logger: zap.NewNop(),
	}

	err := service.dial(context.Background())
	assert.Error(t, err)
	assert.NotNil(t, service.client)
	assert.NotNil(t, service.session)
	assert.Nil(t, service.receiver)
}

func mockWriteData(conn *connMock, data [][]byte, callbacks ...func(sentData, receivedData []byte)) {
	conn.setWriteHandler(func(b []byte) (n int, err error) {
		var next []byte
		if len(data) != 0 {
			next = data[0]
			data = data[1:] // move pointer
		}
		for _, callback := range callbacks {
			callback(b, next)
		}
		if next != nil {
			conn.nextData <- next
		}
		return len(b), nil
	})
}

func mockDialFunc(conn *connMock) {
	dialFunc = func(ctx context.Context, addr string, opts *amqp.ConnOptions) (*amqp.Conn, error) {
		defer func() { dialFunc = amqp.Dial }() // reset dialFunc
		return amqp.NewConn(ctx, conn, opts)
	}
}

// validate that all substitution variables are set correctly
func TestAMQPSubstituteVariables(t *testing.T) {
	testFunctionEquality(t, amqp.SASLTypePlain, connSASLPlain)
	testFunctionEquality(t, amqp.SASLTypeXOAUTH2, connSASLXOAUTH2)
	testFunctionEquality(t, amqp.SASLTypeExternal, connSASLExternal)
	testFunctionEquality(t, amqp.Dial, dialFunc)
}

// testFunctionEquality will check that the pointer names are the same for the two functions.
// It is not a perfect comparison but will perform well differentiating between anonymous
// functions and the amqp named functinos
func testFunctionEquality(t *testing.T, f1, f2 interface{}) {
	assert.True(t, (f1 == nil) == (f2 == nil))
	if f1 == nil {
		return
	}
	funcName1 := runtime.FuncForPC(reflect.ValueOf(f1).Pointer()).Name()
	funcName2 := runtime.FuncForPC(reflect.ValueOf(f2).Pointer()).Name()
	assert.Equal(t, funcName1, funcName2)
}

func TestConfigAMQPAuthenticationPlaintext(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	const username = "uname"
	const password = "pwd"
	cfg.Auth.PlainText = &SaslPlainTextConfig{
		Username: username,
		Password: password,
	}
	defer func() {
		connSASLPlain = amqp.SASLTypePlain
	}()
	called := false
	connSASLPlain = func(passedUsername, passedPassword string) amqp.SASLType {
		assert.Equal(t, username, passedUsername)
		assert.Equal(t, password, passedPassword)
		called = true
		return nil
	}
	_, err := toAMQPAuthentication(cfg)
	assert.NoError(t, err)
	assert.True(t, called)
}

func TestConfigAMQPAuthenticationPlaintextMissingUsername(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Auth.PlainText = &SaslPlainTextConfig{
		Password: "Password",
	}
	result, err := toAMQPAuthentication(cfg)
	assert.Nil(t, result)
	assert.Equal(t, errMissingPlainTextParams, err)
}

func TestConfigAMQPAuthenticationPlaintextMissingPassword(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Auth.PlainText = &SaslPlainTextConfig{
		Username: "Username",
	}
	result, err := toAMQPAuthentication(cfg)
	assert.Nil(t, result)
	assert.Equal(t, errMissingPlainTextParams, err)
}

func TestConfigAMQPAuthenticationXAuth2(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	const username = "uname"
	const bearer = "pwd"
	cfg.Auth.XAuth2 = &SaslXAuth2Config{
		Username: username,
		Bearer:   bearer,
	}
	defer func() {
		connSASLXOAUTH2 = amqp.SASLTypeXOAUTH2
	}()
	called := false
	connSASLXOAUTH2 = func(passedUsername, passedBearer string, maxFrameSizeOverride uint32) amqp.SASLType {
		assert.Equal(t, username, passedUsername)
		assert.Equal(t, bearer, passedBearer)
		assert.EqualValues(t, saslMaxInitFrameSizeOverride, maxFrameSizeOverride)
		called = true
		return nil
	}
	_, err := toAMQPAuthentication(cfg)
	assert.NoError(t, err)
	assert.True(t, called)
}

func TestConfigAMQPAuthenticationXAuth2MissingUsername(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Auth.XAuth2 = &SaslXAuth2Config{
		Bearer: "abc",
	}
	result, err := toAMQPAuthentication(cfg)
	assert.Nil(t, result)
	assert.Equal(t, errMissingXauth2Params, err)
}

func TestConfigAMQPAuthenticationXAuth2MissingBearer(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Auth.XAuth2 = &SaslXAuth2Config{
		Username: "user",
	}
	result, err := toAMQPAuthentication(cfg)
	assert.Nil(t, result)
	assert.Equal(t, errMissingXauth2Params, err)
}

func TestConfigAMQPAuthenticationExternal(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Auth.External = &SaslExternalConfig{}
	defer func() {
		connSASLExternal = amqp.SASLTypeExternal
	}()
	called := false
	connSASLExternal = func(resp string) amqp.SASLType {
		assert.Equal(t, "", resp)
		called = true
		return nil
	}
	_, err := toAMQPAuthentication(cfg)
	assert.NoError(t, err)
	assert.True(t, called)
}

func TestConfigAMQPAuthenticationNoDetails(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	result, err := toAMQPAuthentication(cfg)
	assert.Nil(t, result)
	assert.Equal(t, errMissingAuthDetails, err)
}

// connMock is used to mock out a net.Conn to return data when requested from Read.
// Read will block until data is added to the nextData channel.
// Write will call writeHandle
type connMock struct {
	nextData    chan []byte
	writeHandle unsafe.Pointer // expected type: func([]byte) (n int, err error)
	closeHandle unsafe.Pointer // expected type: func() error
	remaining   *bytes.Reader
}

type writeHandler func([]byte) (n int, err error)
type closeHandler func() error

func (c *connMock) setWriteHandler(handler writeHandler) {
	atomic.StorePointer(&c.writeHandle, unsafe.Pointer(&handler))
}

func (c *connMock) setCloseHandler(handler closeHandler) {
	atomic.StorePointer(&c.closeHandle, unsafe.Pointer(&handler))
}

func (c *connMock) Read(b []byte) (n int, err error) {
	if c.remaining == nil {
		d := <-c.nextData
		// the way this test fixture is designed, there is a race condition
		// between write and read where data may be written to nextData on
		// a call to Write and may be propogated prior to the return of Write.
		time.Sleep(10 * time.Millisecond)
		c.remaining = bytes.NewReader(d)
	}
	defer func() {
		if c.remaining.Len() == 0 {
			c.remaining = nil
		}
	}()
	return c.remaining.Read(b)
}

func (c *connMock) Write(b []byte) (n int, err error) {
	handlerPointer := atomic.LoadPointer(&c.writeHandle)
	if handlerPointer != nil {
		return (*(*writeHandler)(handlerPointer))(b)
	}
	return len(b), nil
}

func (c *connMock) Close() error {
	handlerPointer := atomic.LoadPointer(&c.closeHandle)
	if handlerPointer != nil {
		return (*(*closeHandler)(handlerPointer))()
	}
	return nil
}
func (c *connMock) LocalAddr() net.Addr {
	return nil
}
func (c *connMock) RemoteAddr() net.Addr {
	return nil
}
func (c *connMock) SetDeadline(_ time.Time) error {
	return nil
}
func (c *connMock) SetReadDeadline(_ time.Time) error {
	return nil
}
func (c *connMock) SetWriteDeadline(_ time.Time) error {
	return nil
}

func assertChannelClosed(t *testing.T, c chan struct{}) {
	select {
	case <-c: // success
	case <-time.After(100 * time.Millisecond):
		t.Error("timed out waiting for dial to be called")
	}
}

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package udp // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/udp"

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net"
	"strconv"
	"sync"

	"go.uber.org/zap"
	"golang.org/x/text/encoding"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/decode"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/split"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/trim"
)

const (
	operatorType = "udp_input"

	// Maximum UDP packet size
	MaxUDPSize = 64 * 1024

	defaultReaders        = 1
	defaultProcessors     = 1
	defaultMaxQueueLength = 100
)

func init() {
	operator.Register(operatorType, func() operator.Builder { return NewConfig() })
}

// NewConfig creates a new UDP input config with default values
func NewConfig() *Config {
	return NewConfigWithID(operatorType)
}

// NewConfigWithID creates a new UDP input config with default values
func NewConfigWithID(operatorID string) *Config {
	return &Config{
		InputConfig: helper.NewInputConfig(operatorID, operatorType),
		BaseConfig: BaseConfig{
			Encoding:        "utf-8",
			OneLogPerPacket: false,
			SplitConfig: split.Config{
				LineEndPattern: ".^", // Use never matching regex to not split data by default
			},
		},
	}
}

// Config is the configuration of a udp input operator.
type Config struct {
	helper.InputConfig `mapstructure:",squash"`
	BaseConfig         `mapstructure:",squash"`
}

type AsyncConfig struct {
	Readers        int `mapstructure:"readers,omitempty"`
	Processors     int `mapstructure:"processors,omitempty"`
	MaxQueueLength int `mapstructure:"max_queue_length,omitempty"`
}

// BaseConfig is the details configuration of a udp input operator.
type BaseConfig struct {
	ListenAddress   string       `mapstructure:"listen_address,omitempty"`
	OneLogPerPacket bool         `mapstructure:"one_log_per_packet,omitempty"`
	AddAttributes   bool         `mapstructure:"add_attributes,omitempty"`
	Encoding        string       `mapstructure:"encoding,omitempty"`
	SplitConfig     split.Config `mapstructure:"multiline,omitempty"`
	TrimConfig      trim.Config  `mapstructure:",squash"`
	AsyncConfig     *AsyncConfig `mapstructure:"async,omitempty"`
}

// Build will build a udp input operator.
func (c Config) Build(logger *zap.SugaredLogger) (operator.Operator, error) {
	inputOperator, err := c.InputConfig.Build(logger)
	if err != nil {
		return nil, err
	}

	if c.ListenAddress == "" {
		return nil, fmt.Errorf("missing required parameter 'listen_address'")
	}

	address, err := net.ResolveUDPAddr("udp", c.ListenAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve listen_address: %w", err)
	}

	enc, err := decode.LookupEncoding(c.Encoding)
	if err != nil {
		return nil, err
	}

	// Build split func
	splitFunc, err := c.SplitConfig.Func(enc, true, MaxUDPSize)
	if err != nil {
		return nil, err
	}
	splitFunc = trim.WithFunc(splitFunc, c.TrimConfig.Func())

	var resolver *helper.IPResolver
	if c.AddAttributes {
		resolver = helper.NewIPResolver()
	}

	if c.AsyncConfig != nil {
		if c.AsyncConfig.Readers <= 0 {
			c.AsyncConfig.Readers = defaultReaders
		}
		if c.AsyncConfig.Processors <= 0 {
			c.AsyncConfig.Processors = defaultProcessors
		}
		if c.AsyncConfig.MaxQueueLength <= 0 {
			c.AsyncConfig.MaxQueueLength = defaultMaxQueueLength
		}
	}

	udpInput := &Input{
		InputOperator:   inputOperator,
		address:         address,
		buffer:          make([]byte, MaxUDPSize),
		addAttributes:   c.AddAttributes,
		encoding:        enc,
		splitFunc:       splitFunc,
		resolver:        resolver,
		OneLogPerPacket: c.OneLogPerPacket,
		AsyncConfig:     c.AsyncConfig,
	}

	if c.AsyncConfig != nil {
		udpInput.messageQueue = make(chan messageAndAddress, c.AsyncConfig.MaxQueueLength)
		udpInput.readBufferPool = sync.Pool{
			New: func() any {
				buffer := make([]byte, MaxUDPSize)
				return &buffer
			},
		}
	}
	return udpInput, nil
}

// Input is an operator that listens to a socket for log entries.
type Input struct {
	buffer []byte
	helper.InputOperator
	address         *net.UDPAddr
	addAttributes   bool
	OneLogPerPacket bool
	AsyncConfig     *AsyncConfig

	connection net.PacketConn
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	wgReader   sync.WaitGroup

	encoding  encoding.Encoding
	splitFunc bufio.SplitFunc
	resolver  *helper.IPResolver

	messageQueue   chan messageAndAddress
	readBufferPool sync.Pool
	stopOnce       sync.Once
}

type messageAndAddress struct {
	Message       *[]byte
	RemoteAddr    net.Addr
	MessageLength int
}

// Start will start listening for messages on a socket.
func (u *Input) Start(_ operator.Persister) error {
	ctx, cancel := context.WithCancel(context.Background())
	u.cancel = cancel

	conn, err := net.ListenUDP("udp", u.address)
	if err != nil {
		return fmt.Errorf("failed to open connection: %w", err)
	}
	u.connection = conn

	u.goHandleMessages(ctx)
	return nil
}

// goHandleMessages will handle messages from a udp connection.
func (u *Input) goHandleMessages(ctx context.Context) {
	if u.AsyncConfig == nil {
		u.wg.Add(1)
		go u.readAndProcessMessages(ctx)
		return
	}

	for i := 0; i < u.AsyncConfig.Readers; i++ {
		u.wgReader.Add(1)
		go u.readMessagesAsync(ctx)
	}

	for i := 0; i < u.AsyncConfig.Processors; i++ {
		u.wg.Add(1)
		go u.processMessagesAsync(ctx)
	}
}

func (u *Input) readAndProcessMessages(ctx context.Context) {
	defer u.wg.Done()

	dec := decode.New(u.encoding)
	readBuffer := make([]byte, MaxUDPSize)
	scannerBuffer := make([]byte, 0, MaxUDPSize)
	for {
		message, remoteAddr, bufferLength, err := u.readMessage(readBuffer)
		message = u.removeTrailingCharactersAndNULsFromBuffer(message, bufferLength)

		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				u.Errorw("Failed reading messages", zap.Error(err))
			}
			break
		}

		u.processMessage(ctx, message, remoteAddr, dec, scannerBuffer)
	}
}

func (u *Input) processMessage(ctx context.Context, message []byte, remoteAddr net.Addr, dec *decode.Decoder, scannerBuffer []byte) {
	if u.OneLogPerPacket {
		log := truncateMaxLog(message)
		u.handleMessage(ctx, remoteAddr, dec, log)
		return
	}

	scanner := bufio.NewScanner(bytes.NewReader(message))
	scanner.Buffer(scannerBuffer, MaxUDPSize)

	scanner.Split(u.splitFunc)

	for scanner.Scan() {
		u.handleMessage(ctx, remoteAddr, dec, scanner.Bytes())
	}
	if err := scanner.Err(); err != nil {
		u.Errorw("Scanner error", zap.Error(err))
	}
}

func (u *Input) readMessagesAsync(ctx context.Context) {
	defer u.wgReader.Done()

	for {
		readBuffer := u.readBufferPool.Get().(*[]byte) // Can't reuse the same buffer since same references would be written multiple times to the messageQueue (and cause data override of previous entries)
		message, remoteAddr, bufferLength, err := u.readMessage(*readBuffer)
		if err != nil {
			u.readBufferPool.Put(readBuffer)
			select {
			case <-ctx.Done():
				return
			default:
				u.Errorw("Failed reading messages", zap.Error(err))
			}
			break
		}

		messageAndAddr := messageAndAddress{
			Message:       &message,
			MessageLength: bufferLength,
			RemoteAddr:    remoteAddr,
		}

		// Send the message to the message queue for processing
		u.messageQueue <- messageAndAddr
	}
}

func (u *Input) processMessagesAsync(ctx context.Context) {
	defer u.wg.Done()

	dec := decode.New(u.encoding)
	scannerBuffer := make([]byte, 0, MaxUDPSize)

	for {
		// Read a message from the message queue.
		messageAndAddr, ok := <-u.messageQueue
		if !ok {
			return // Channel closed, exit the goroutine.
		}

		trimmedMessage := u.removeTrailingCharactersAndNULsFromBuffer(*messageAndAddr.Message, messageAndAddr.MessageLength)
		u.processMessage(ctx, trimmedMessage, messageAndAddr.RemoteAddr, dec, scannerBuffer)
		u.readBufferPool.Put(messageAndAddr.Message)
	}
}

func truncateMaxLog(data []byte) (token []byte) {
	if len(data) >= MaxUDPSize {
		return data[:MaxUDPSize]
	}

	if len(data) == 0 {
		return nil
	}

	return data
}

func (u *Input) handleMessage(ctx context.Context, remoteAddr net.Addr, dec *decode.Decoder, log []byte) {
	decoded := log
	if u.encoding != encoding.Nop {
		var err error
		decoded, err = dec.Decode(log)
		if err != nil {
			u.Errorw("Failed to decode data", zap.Error(err))
			return
		}
	}

	entry, err := u.NewEntry(string(decoded))
	if err != nil {
		u.Errorw("Failed to create entry", zap.Error(err))
		return
	}

	if u.addAttributes {
		entry.AddAttribute("net.transport", "IP.UDP")
		if addr, ok := u.connection.LocalAddr().(*net.UDPAddr); ok {
			ip := addr.IP.String()
			entry.AddAttribute("net.host.ip", addr.IP.String())
			entry.AddAttribute("net.host.port", strconv.FormatInt(int64(addr.Port), 10))
			entry.AddAttribute("net.host.name", u.resolver.GetHostFromIP(ip))
		}

		if addr, ok := remoteAddr.(*net.UDPAddr); ok {
			ip := addr.IP.String()
			entry.AddAttribute("net.peer.ip", ip)
			entry.AddAttribute("net.peer.port", strconv.FormatInt(int64(addr.Port), 10))
			entry.AddAttribute("net.peer.name", u.resolver.GetHostFromIP(ip))
		}
	}

	u.Write(ctx, entry)
}

// readMessage will read log messages from the connection.
func (u *Input) readMessage(buffer []byte) ([]byte, net.Addr, int, error) {
	n, addr, err := u.connection.ReadFrom(buffer)
	if err != nil {
		return nil, nil, 0, err
	}

	return buffer, addr, n, nil
}

// This will remove trailing characters and NULs from the buffer
func (u *Input) removeTrailingCharactersAndNULsFromBuffer(buffer []byte, n int) []byte {
	// Remove trailing characters and NULs
	for ; (n > 0) && (buffer[n-1] < 32); n-- { // nolint
	}

	return buffer[:n]
}

// Stop will stop listening for udp messages.
func (u *Input) Stop() error {
	u.stopOnce.Do(func() {
		if u.cancel == nil {
			return
		}
		u.cancel()
		if u.connection != nil {
			if err := u.connection.Close(); err != nil {
				u.Errorf("failed to close UDP connection: %s", err)
			}
		}
		if u.AsyncConfig != nil {
			u.wgReader.Wait() // only when all async readers are finished, so there's no risk of sending to a closed channel, do we close messageQueue (which allows the async processors to finish)
			close(u.messageQueue)
		}

		u.wg.Wait()
		if u.resolver != nil {
			u.resolver.Stop()
		}
	})
	return nil
}

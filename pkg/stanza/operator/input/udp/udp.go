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
	"time"

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
			AsyncConcurrentMode:             false,
			FixedAsyncReaderRoutineCount:    1,
			FixedAsyncProcessorRoutineCount: 1,
			MaxAsyncQueueLength:             1000,
			MaxGracefulShutdownTimeInMS:     1000,
		},
	}
}

// Config is the configuration of a udp input operator.
type Config struct {
	helper.InputConfig `mapstructure:",squash"`
	BaseConfig         `mapstructure:",squash"`
}

// BaseConfig is the details configuration of a udp input operator.
type BaseConfig struct {
	ListenAddress                   string       `mapstructure:"listen_address,omitempty"`
	OneLogPerPacket                 bool         `mapstructure:"one_log_per_packet,omitempty"`
	AddAttributes                   bool         `mapstructure:"add_attributes,omitempty"`
	Encoding                        string       `mapstructure:"encoding,omitempty"`
	SplitConfig                     split.Config `mapstructure:"multiline,omitempty"`
	TrimConfig                      trim.Config  `mapstructure:",squash"`
	AsyncConcurrentMode             bool         `mapstructure:"async_concurrent_mode,omitempty"`
	FixedAsyncReaderRoutineCount    int          `mapstructure:"fixed_async_reader_routine_count,omitempty"`
	FixedAsyncProcessorRoutineCount int          `mapstructure:"fixed_async_processor_routine_count,omitempty"`
	MaxAsyncQueueLength             int          `mapstructure:"max_async_queue_length,omitempty"`
	MaxGracefulShutdownTimeInMS     int          `mapstructure:"max_graceful_shutdown_time_in_ms,omitempty"`
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

	udpInput := &Input{
		InputOperator:                   inputOperator,
		address:                         address,
		buffer:                          make([]byte, MaxUDPSize),
		addAttributes:                   c.AddAttributes,
		encoding:                        enc,
		splitFunc:                       splitFunc,
		resolver:                        resolver,
		OneLogPerPacket:                 c.OneLogPerPacket,
		AsyncConcurrentMode:             c.AsyncConcurrentMode,
		FixedAsyncReaderRoutineCount:    c.FixedAsyncReaderRoutineCount,
		FixedAsyncProcessorRoutineCount: c.FixedAsyncProcessorRoutineCount,
		MaxGracefulShutdownTimeInMS:     c.MaxGracefulShutdownTimeInMS,
		messageQueue:                    make(chan MessageAndAddr, c.MaxAsyncQueueLength),
		shutdownChan:                    make(chan struct{}),
	}
	return udpInput, nil
}

// Input is an operator that listens to a socket for log entries.
type Input struct {
	buffer []byte
	helper.InputOperator
	address                         *net.UDPAddr
	addAttributes                   bool
	OneLogPerPacket                 bool
	AsyncConcurrentMode             bool
	FixedAsyncReaderRoutineCount    int
	FixedAsyncProcessorRoutineCount int
	MaxGracefulShutdownTimeInMS     int

	connection net.PacketConn
	cancel     context.CancelFunc
	wg         sync.WaitGroup

	encoding  encoding.Encoding
	splitFunc bufio.SplitFunc
	resolver  *helper.IPResolver

	messageQueue chan MessageAndAddr
	shutdownChan chan struct{}
	stopOnce sync.Once
}

type MessageAndAddr struct {
	Message    []byte
	RemoteAddr net.Addr
}

// Start will start listening for messages on a socket.
func (u *Input) Start(_ operator.Persister) error {
	ctx, cancel := context.WithCancel(context.Background())
	u.cancel = cancel

	u.wg = sync.WaitGroup{}

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
	if u.AsyncConcurrentMode {
		// Start the specified number of goroutines
		for i := 0; i < u.FixedAsyncReaderRoutineCount; i++ {
			u.wg.Add(1)
			go u.readMessagesAsync(ctx)
		}

		for i := 0; i < u.FixedAsyncProcessorRoutineCount; i++ {
			u.wg.Add(1)
			go u.processMessagesAsync(ctx)
		}
	} else {
		u.wg.Add(1)
		go u.readAndProcessMessages(ctx)
	}
}

func (u *Input) readAndProcessMessages(ctx context.Context) {
	defer u.wg.Done()

	dec := decode.New(u.encoding)
	buf := make([]byte, 0, MaxUDPSize)

	for {
		message, remoteAddr, err := u.readMessage()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				u.Errorw("Failed reading messages", zap.Error(err))
			}
			break
		}

		u.processMessage(ctx, message, remoteAddr, dec, buf)
	}
}

func (u *Input) processMessage(ctx context.Context, message []byte, remoteAddr net.Addr, dec *decode.Decoder, buf []byte) {
	if u.OneLogPerPacket {
		log := truncateMaxLog(message)
		u.handleMessage(ctx, remoteAddr, dec, log)
		return
	}

	scanner := bufio.NewScanner(bytes.NewReader(message))
	scanner.Buffer(buf, MaxUDPSize)
	scanner.Split(u.splitFunc)

	for scanner.Scan() {
		u.handleMessage(ctx, remoteAddr, dec, scanner.Bytes())
	}
	if err := scanner.Err(); err != nil {
		u.Errorw("Scanner error", zap.Error(err))
	}
}

func (u *Input) readMessagesAsync(ctx context.Context) {
	defer u.wg.Done()

	for {
		select {
		case <-u.shutdownChan:
			return // Exit gracefully if shutdown is initiated. No need to read more messages from UDP.
		default:
			message, remoteAddr, err := u.readMessage()
			if err != nil {
				select {
				case <-ctx.Done():
					return
				default:
					u.Errorw("Failed reading messages", zap.Error(err))
				}
				break
			}

			messageAndAddr := MessageAndAddr{
				Message:    message,
				RemoteAddr: remoteAddr,
			}

			// Send the message to the message queue for processing
			u.messageQueue <- messageAndAddr
		}
	}
}

func (u *Input) processMessagesAsync(ctx context.Context) {
	defer u.wg.Done()

	dec := decode.New(u.encoding)
	buf := make([]byte, 0, MaxUDPSize)

	for {
		// Read a message from the message queue.
		messageAndAddr, ok := <-u.messageQueue
		if !ok {
			// Channel closed, exit the goroutine. Note that the channel will only be closed (by the Stop method during shutdown) once messageQueue is empty.
			// Until then, processMessagesAsync will keep reading & processing messages from the queue to reduce data-loss.
			return
		}

		u.processMessage(ctx, messageAndAddr.Message, messageAndAddr.RemoteAddr, dec, buf)
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
	decoded, err := dec.Decode(log)
	if err != nil {
		u.Errorw("Failed to decode data", zap.Error(err))
		return
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
func (u *Input) readMessage() ([]byte, net.Addr, error) {
	n, addr, err := u.connection.ReadFrom(u.buffer)
	if err != nil {
		return nil, nil, err
	}

	// Remove trailing characters and NULs
	for ; (n > 0) && (u.buffer[n-1] < 32); n-- { // nolint
	}

	return u.buffer[:n], addr, nil
}

// Stop will stop listening for udp messages.
func (u *Input) Stop() error {
	// Using sync.Once to ensure the Stop method is called only once
    u.stopOnce.Do(func() {
		// Signal shutdown. This would cause the readAsync routine to stop reading from UDP port.
		close(u.shutdownChan)

		if u.AsyncConcurrentMode {
			startTime := time.Now()

			// Wait for the processAsync method to finish reading & processing all messages from messageQueue. In other words, wait for the channel to be empty, but don't read from it.
			for len(u.messageQueue) > 0 {
				if time.Since(startTime) > time.Duration(u.MaxGracefulShutdownTimeInMS)*time.Millisecond {
					u.Errorf("Stop timed out waiting for messageQueue to become empty. Shutting down despite messageQueue still having messages to process.")
					break
				}

				select {
				case <-time.After(10 * time.Millisecond):
					// Sleep for 10 milliseconds before checking whehther messageQueue is empty again
				default:
				}
			}

			close(u.messageQueue)
		}

		if u.cancel != nil {
			u.cancel()
		}

		if u.connection != nil {
			if err := u.connection.Close(); err != nil {
				u.Errorf("failed to close UDP connection: %s", err)
			}
		}
		u.wg.Wait()
		if u.resolver != nil {
			u.resolver.Stop()
		}
	})

	return nil
}

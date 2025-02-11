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
)

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
func (i *Input) Start(_ operator.Persister) error {
	ctx, cancel := context.WithCancel(context.Background())
	i.cancel = cancel

	conn, err := net.ListenUDP("udp", i.address)
	if err != nil {
		return fmt.Errorf("failed to open connection: %w", err)
	}
	i.connection = conn

	i.goHandleMessages(ctx)
	return nil
}

// goHandleMessages will handle messages from a udp connection.
func (i *Input) goHandleMessages(ctx context.Context) {
	if i.AsyncConfig == nil {
		i.wg.Add(1)
		go i.readAndProcessMessages(ctx)
		return
	}

	for n := 0; n < i.AsyncConfig.Readers; n++ {
		i.wgReader.Add(1)
		go i.readMessagesAsync(ctx)
	}

	for n := 0; n < i.AsyncConfig.Processors; n++ {
		i.wg.Add(1)
		go i.processMessagesAsync(ctx)
	}
}

func (i *Input) readAndProcessMessages(ctx context.Context) {
	defer i.wg.Done()

	dec := decode.New(i.encoding)
	readBuffer := make([]byte, MaxUDPSize)
	scannerBuffer := make([]byte, 0, MaxUDPSize)
	for {
		message, remoteAddr, bufferLength, err := i.readMessage(readBuffer)
		message = i.removeTrailingCharactersAndNULsFromBuffer(message, bufferLength)

		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				i.Logger().Error("Failed reading messages", zap.Error(err))
			}
			break
		}

		i.processMessage(ctx, message, remoteAddr, dec, scannerBuffer)
	}
}

func (i *Input) processMessage(ctx context.Context, message []byte, remoteAddr net.Addr, dec *decode.Decoder, scannerBuffer []byte) {
	if i.OneLogPerPacket {
		log := truncateMaxLog(message)
		i.handleMessage(ctx, remoteAddr, dec, log)
		return
	}

	scanner := bufio.NewScanner(bytes.NewReader(message))
	scanner.Buffer(scannerBuffer, MaxUDPSize)

	scanner.Split(i.splitFunc)

	for scanner.Scan() {
		i.handleMessage(ctx, remoteAddr, dec, scanner.Bytes())
	}
	if err := scanner.Err(); err != nil {
		i.Logger().Error("Scanner error", zap.Error(err))
	}
}

func (i *Input) readMessagesAsync(ctx context.Context) {
	defer i.wgReader.Done()

	for {
		readBuffer := i.readBufferPool.Get().(*[]byte) // Can't reuse the same buffer since same references would be written multiple times to the messageQueue (and cause data override of previous entries)
		message, remoteAddr, bufferLength, err := i.readMessage(*readBuffer)
		if err != nil {
			i.readBufferPool.Put(readBuffer)
			select {
			case <-ctx.Done():
				return
			default:
				i.Logger().Error("Failed reading messages", zap.Error(err))
			}
			break
		}

		messageAndAddr := messageAndAddress{
			Message:       &message,
			MessageLength: bufferLength,
			RemoteAddr:    remoteAddr,
		}

		// Send the message to the message queue for processing
		i.messageQueue <- messageAndAddr
	}
}

func (i *Input) processMessagesAsync(ctx context.Context) {
	defer i.wg.Done()

	dec := decode.New(i.encoding)
	scannerBuffer := make([]byte, 0, MaxUDPSize)

	for {
		// Read a message from the message queue.
		messageAndAddr, ok := <-i.messageQueue
		if !ok {
			return // Channel closed, exit the goroutine.
		}

		trimmedMessage := i.removeTrailingCharactersAndNULsFromBuffer(*messageAndAddr.Message, messageAndAddr.MessageLength)
		i.processMessage(ctx, trimmedMessage, messageAndAddr.RemoteAddr, dec, scannerBuffer)
		i.readBufferPool.Put(messageAndAddr.Message)
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

func (i *Input) handleMessage(ctx context.Context, remoteAddr net.Addr, dec *decode.Decoder, log []byte) {
	decoded := log
	if i.encoding != encoding.Nop {
		var err error
		decoded, err = dec.Decode(log)
		if err != nil {
			i.Logger().Error("Failed to decode data", zap.Error(err))
			return
		}
	}

	entry, err := i.NewEntry(string(decoded))
	if err != nil {
		i.Logger().Error("Failed to create entry", zap.Error(err))
		return
	}

	if i.addAttributes {
		entry.AddAttribute("net.transport", "IP.UDP")
		if addr, ok := i.connection.LocalAddr().(*net.UDPAddr); ok {
			ip := addr.IP.String()
			entry.AddAttribute("net.host.ip", addr.IP.String())
			entry.AddAttribute("net.host.port", strconv.FormatInt(int64(addr.Port), 10))
			entry.AddAttribute("net.host.name", i.resolver.GetHostFromIP(ip))
		}

		if addr, ok := remoteAddr.(*net.UDPAddr); ok {
			ip := addr.IP.String()
			entry.AddAttribute("net.peer.ip", ip)
			entry.AddAttribute("net.peer.port", strconv.FormatInt(int64(addr.Port), 10))
			entry.AddAttribute("net.peer.name", i.resolver.GetHostFromIP(ip))
		}
	}

	err = i.Write(ctx, entry)
	if err != nil {
		i.Logger().Error("Failed to write entry", zap.Error(err))
	}
}

// readMessage will read log messages from the connection.
func (i *Input) readMessage(buffer []byte) ([]byte, net.Addr, int, error) {
	n, addr, err := i.connection.ReadFrom(buffer)
	if err != nil {
		return nil, nil, 0, err
	}

	return buffer, addr, n, nil
}

// This will remove trailing characters and NULs from the buffer
func (i *Input) removeTrailingCharactersAndNULsFromBuffer(buffer []byte, n int) []byte {
	// Remove trailing characters and NULs
	for ; (n > 0) && (buffer[n-1] < 32); n-- { // nolint
	}

	return buffer[:n]
}

// Stop will stop listening for udp messages.
func (i *Input) Stop() error {
	i.stopOnce.Do(func() {
		if i.cancel == nil {
			return
		}
		i.cancel()
		if i.connection != nil {
			if err := i.connection.Close(); err != nil {
				i.Logger().Error("failed to close UDP connection", zap.Error(err))
			}
		}
		if i.AsyncConfig != nil {
			i.wgReader.Wait() // only when all async readers are finished, so there's no risk of sending to a closed channel, do we close messageQueue (which allows the async processors to finish)
			close(i.messageQueue)
		}

		i.wg.Wait()
		if i.resolver != nil {
			i.resolver.Stop()
		}
	})
	return nil
}

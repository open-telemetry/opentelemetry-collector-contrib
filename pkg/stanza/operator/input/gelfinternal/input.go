package gelfinternal

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

const (
	ChunkSize        = 1420
	chunkedHeaderLen = 12
	chunkedDataLen   = ChunkSize - chunkedHeaderLen
	chunkHeaderSize  = 12
)

var (
	magicChunked = []byte{0x1e, 0x0f}
	magicZlib    = []byte{0x78}
	magicGzip    = []byte{0x1f, 0x8b}
)
type Input struct {
	helper.InputOperator
	wg              sync.WaitGroup
	cancel          context.CancelFunc
	address         string
	protocol        string
	conn            net.PacketConn
	readBufferPool  sync.Pool
	udpMessageQueue chan UDPMessage
	wgReader        sync.WaitGroup
	wgProcessor     sync.WaitGroup
	asyncReaders    int
	asyncProcessors int

	buffer     map[string]*MapGelfMessage
	lastBuffer map[string]*MapGelfMessage
	lastSwap   time.Time
	muBuffer   sync.Mutex
}

type GELFSegment struct {
	Id             string
	SequenceNumber int
	TotalCount     int
	Data           []byte
}

type Message struct {
	Version  string                 `json:"version"`
	Host     string                 `json:"host"`
	Short    string                 `json:"short_message"`
	Full     string                 `json:"full_message,omitempty"`
	TimeUnix float64                `json:"timestamp"`
	Level    int32                  `json:"level,omitempty"`
	Facility string                 `json:"facility,omitempty"`
	Extra    map[string]interface{} `json:"-"`
	RawExtra json.RawMessage        `json:"-"`
}

type UDPMessage struct {
	length int
	buffer []byte
	addr   net.Addr
}

type MapGelfMessage struct {
	stored   int
	segments [128][]byte
}

// Start will start listening for messages on a socket.
func (gelfRcvInput *Input) Start(_ operator.Persister) error {
	ctx, cancel := context.WithCancel(context.Background())
	gelfRcvInput.cancel = cancel

	udpAddr, err := net.ResolveUDPAddr(gelfRcvInput.protocol, gelfRcvInput.address)
	if err != nil {
		return fmt.Errorf("ResolveUDPAddr('%s'): %s", gelfRcvInput.address, err)
	}

	conn, err := net.ListenUDP(gelfRcvInput.protocol, udpAddr)
	if err != nil {
		gelfRcvInput.Logger().Error("Failed to open connection", zap.Error(err))
		return fmt.Errorf("failed to open connection: %w", err)
	}

	gelfRcvInput.Logger().Info("Started GELF UDP server")

	gelfRcvInput.conn = conn

	go gelfRcvInput.startReading(ctx)

	gelfRcvInput.Logger().Debug("Exiting the _Start function")

	return nil
}

// Stop will stop listening for udp messages.
func (gelfRcvInput *Input) Stop() error {
	if gelfRcvInput.cancel == nil {
		return nil
	}
	gelfRcvInput.cancel()
	gelfRcvInput.conn.Close()
	gelfRcvInput.Logger().Info("Stopping GELF server")
	gelfRcvInput.wg.Wait()
	close(gelfRcvInput.udpMessageQueue)

	return nil
}

// GELF code
// https://github.com/Graylog2/go-gelf/blob/master/gelf/gelf.go

func (gelfRcvInput *Input) GelfNewReader(addr string) error {
	var err error
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return fmt.Errorf("ResolveUDPAddr('%s'): %s", addr, err)
	}

	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return fmt.Errorf("ListenUDP: %s", err)
	}
	gelfRcvInput.conn = conn
	return nil
}

func (gelfRcvInput *Input) startReading(ctx context.Context) {
	for n := 0; n < gelfRcvInput.asyncProcessors; n++ {
		gelfRcvInput.wgReader.Add(1)
		go gelfRcvInput.ReadUDPBufferAsync(ctx)
		gelfRcvInput.Logger().Debug("Started read workers...")
	}
	for n := 0; n < gelfRcvInput.asyncProcessors; n++ {
		gelfRcvInput.wgProcessor.Add(1)
		go gelfRcvInput.processMessagesAsync(ctx)
		gelfRcvInput.Logger().Debug("Started processor workers...")
	}
}

func (gelfRcvInput *Input) ReadUDPBufferAsync(ctx context.Context) {

	defer gelfRcvInput.wgReader.Done()

	for {

		var (
			n    int
			err  error
			addr net.Addr
		)
		buf := make([]byte, ChunkSize)

		n, addr, err = gelfRcvInput.conn.ReadFrom(buf)

		gelfRcvInput.Logger().Debug("Received message from UDP")

		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				gelfRcvInput.Logger().Error("Failed reading messages", zap.Error(err))
			}

			break
		}

		udpMessage := UDPMessage{
			addr:   addr,
			length: n,
			buffer: buf[:n],
		}

		// Send message to udp queue
		gelfRcvInput.udpMessageQueue <- udpMessage
	}
}

func (gelfRcvInput *Input) processMessagesAsync(ctx context.Context) {

	defer gelfRcvInput.wgProcessor.Done()

	for {
		// Read a message from the message queue.
		message, ok := <-gelfRcvInput.udpMessageQueue
		gelfRcvInput.Logger().Debug("Received queued message!")
		if !ok {
			return // Channel closed, exit the goroutine.
		}

		gelfRcvInput.Logger().Debug("Received message from UDP - entering Handling gelf message")
		gelfRcvInput.HandleGELFMessage(ctx, message.buffer, message.length)
	}
}

func (gelfRcvInput *Input) HandleGELFMessage(ctx context.Context, packet []byte, n int) {

	// Check if the packet is a chunked message or a full message
	if bytes.Equal(packet[:2], magicChunked) {
		segment := GELFSegment{
			Id:             string(packet[2:10]),
			SequenceNumber: int(packet[10]),
			TotalCount:     int(packet[11]),
			Data:           append(make([]byte, 0, n-len(packet)), packet[chunkedHeaderLen:]...),
		}

		gelfRcvInput.handleChunkedMessage(ctx, &segment, n) // Handle as a chunked message
	} else {

		message, err := gelfRcvInput.extractLog(ctx, packet) // Handle as a full message
		if err != nil {
			gelfRcvInput.Logger().Error("Error during decompression(non-chunked)", zap.Error(err))
			// return nil
		}

		if message.Full == "" {
			gelfRcvInput.writeLog(ctx, message.Short)
		} else {
			gelfRcvInput.writeLog(ctx, message.Full)
		}
	}
}

// handleChunkedMessage processes a chunked GELF message
func (gelfRcvInput *Input) handleChunkedMessage(ctx context.Context, gelfSegment *GELFSegment, n int) {

	gelfRcvInput.muBuffer.Lock()
	defer gelfRcvInput.muBuffer.Unlock()

	// Condition for checking 5 seconds by comparing with last_swap, clear last_buffer and swap current buffer with last_buffer
	// Took inspiration from Tremor's gelf_chunking implementation: https://github.com/tremor-rs/tremor-runtime/blob/main/tremor-interceptor/src/preprocessor/gelf_chunking.rs
	if time.Since(gelfRcvInput.lastSwap) > 5*time.Second {
		gelfRcvInput.lastSwap = time.Now()
		gelfRcvInput.lastBuffer = gelfRcvInput.buffer
		gelfRcvInput.buffer = make(map[string]*MapGelfMessage)
		gelfRcvInput.Logger().Debug("Swapped buffers!")
	}

	// If segment exists in last_buffer then process in that.
	gelfMessage, segmentExists := gelfRcvInput.lastBuffer[gelfSegment.Id]

	if segmentExists {
		if len(gelfMessage.segments[gelfSegment.SequenceNumber]) == 0 {
			// If the sequenceNumber is available then add
			if (gelfSegment.SequenceNumber >= 0) && (gelfSegment.SequenceNumber < gelfSegment.TotalCount) {
				// gelfRcvInput.lastBuffer[gelfSegment.Id] = ni

				gelfMessage.segments[gelfSegment.SequenceNumber] = gelfSegment.Data

				gelfMessage.stored++

				gelfRcvInput.Logger().Debug("last_buffer details", zap.Int("Stored in last_buffer: ", gelfMessage.stored), zap.Int("Total count in last_buffer: ", gelfSegment.TotalCount))

				if gelfMessage.stored == gelfSegment.TotalCount {

					var completeLogBytes []byte
					for i := 0; i < gelfSegment.TotalCount; i++ {
						completeLogBytes = append(completeLogBytes, gelfMessage.segments[i]...)
					}

					message, err := gelfRcvInput.extractLog(ctx, completeLogBytes) // Handle as a full message
					if err != nil {
						gelfRcvInput.Logger().Error("Error during decompression(non-chunked)", zap.Error(err))
					} else {
						if message.Full == "" {
							gelfRcvInput.writeLog(ctx, message.Short)
						} else {
							gelfRcvInput.writeLog(ctx, message.Full)
						}
					}

					// Discarding the chunk even if the
					delete(gelfRcvInput.lastBuffer, gelfSegment.Id)
				}

			} else {
				// if sequence number is not present in segment's range then log error and discard the key from map
				gelfRcvInput.Logger().Error("Discarding out of range chunk")
				delete(gelfRcvInput.lastBuffer, gelfSegment.Id)
			}
		} else {
			gelfRcvInput.Logger().Error("Duplicate index in segment")
		}
	} else {
		// if segment is not present in last_buffer then check current_buffer
		gelfMessage, segmentExists := gelfRcvInput.buffer[gelfSegment.Id]
		if segmentExists {
			if len(gelfMessage.segments[gelfSegment.SequenceNumber]) == 0 {
				// If the sequenceNumber is available then add
				if (gelfSegment.SequenceNumber >= 0) && (gelfSegment.SequenceNumber < gelfSegment.TotalCount) {
					// gelfRcvInput.lastBuffer[gelfSegment.Id] = ni
					gelfMessage.segments[gelfSegment.SequenceNumber] = gelfSegment.Data

					gelfMessage.stored++

					gelfRcvInput.Logger().Debug("current_buffer details", zap.Int("Stored in last_buffer: ", gelfMessage.stored), zap.Int("Total count in last_buffer: ", gelfSegment.TotalCount))

					if gelfMessage.stored == gelfSegment.TotalCount {

						var completeLogBytes []byte
						for i := 0; i < gelfSegment.TotalCount; i++ {
							completeLogBytes = append(completeLogBytes, gelfMessage.segments[i]...)
						}

						message, err := gelfRcvInput.extractLog(ctx, completeLogBytes) // Handle as a full message
						if err != nil {
							gelfRcvInput.Logger().Error("Error during decompression(non-chunked)", zap.Error(err))
							// return nil
						} else {
							if message.Full == "" {
								gelfRcvInput.writeLog(ctx, message.Short)
							} else {
								gelfRcvInput.writeLog(ctx, message.Full)
							}
						}

						delete(gelfRcvInput.lastBuffer, gelfSegment.Id)
					}

				} else {
					// if sequence number is not present in segment's range then log error and discard the key from map
					gelfRcvInput.Logger().Error("Discarding out of range chunk")
					delete(gelfRcvInput.lastBuffer, gelfSegment.Id)
				}
			} else {
				gelfRcvInput.Logger().Error("Duplicate index in segment")
			}
		} else {
			// if segment is not present in current_buffer then add to current_buffer map
			newGelfMessage := MapGelfMessage{
				stored:   0,
				segments: [128][]byte{},
			}

			newGelfMessage.segments[gelfSegment.SequenceNumber] = gelfSegment.Data

			newGelfMessage.stored++

			gelfRcvInput.buffer[gelfSegment.Id] = &newGelfMessage
		}
	}
}

func (gelfRcvInput *Input) extractLog(ctx context.Context, data []byte) (*Message, error) {

	var decompressedDataReader io.ReadCloser
	var reader io.Reader
	var noCompression = false
	var err error

	// Check the compression type and decompress accordingly
	if bytes.HasPrefix(data, magicGzip) {
		decompressedDataReader, err = gzip.NewReader(bytes.NewReader(data))
	} else if bytes.HasPrefix(data, magicZlib) &&
		(int(data[0])*256+int(data[1]))%31 == 0 {
		decompressedDataReader, err = zlib.NewReader(bytes.NewReader(data))
	} else {
		reader = bytes.NewReader(data) // No compression
		noCompression = true
	}

	if err != nil {
		gelfRcvInput.Logger().Error("Error decompressing packet", zap.Error(err))
		return nil, err
	}

	// Write the decompressed message
	msg := new(Message)

	if noCompression {
		if err := json.NewDecoder(reader).Decode(&msg); err != nil {
			return nil, fmt.Errorf("json.Unmarshal: %s", err)
		}
	} else {

		if err := json.NewDecoder(decompressedDataReader).Decode(&msg); err != nil {
			return nil, fmt.Errorf("json.Unmarshal: %s", err)
		}

		decompressedDataReader.Close()
	}

	return msg, nil
}

func (gelfRcvInput *Input) writeLog(ctx context.Context, logMessage string) {

	entry, err := gelfRcvInput.NewEntry(string(logMessage))
	if err != nil {
		gelfRcvInput.Logger().Error("Error creating log entry", zap.Error(err))
		// return nil
	}

	gelfRcvInput.Write(ctx, entry)
}

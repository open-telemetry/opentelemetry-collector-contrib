// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pulsarreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/pulsarreceiver"

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/apache/pulsar-client-go/pulsar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/textutils"
)

func Test_newTracesReceiver_err(t *testing.T) {
	c := Config{
		Encoding: defaultEncoding,
	}
	_, err := newTracesReceiver(c, receivertest.NewNopCreateSettings(), defaultTracesUnmarshalers(), consumertest.NewNop())
	assert.Error(t, err)
}

type testPulsarConsumer struct {
	messageChan chan pulsar.ConsumerMessage
}

func (c testPulsarConsumer) Subscription() string {
	panic("implement me")
}

func (c testPulsarConsumer) Unsubscribe() error {
	panic("implement me")
}

func (c testPulsarConsumer) Receive(_ context.Context) (pulsar.Message, error) {
	msg, ok := <-c.messageChan
	if !ok {
		return nil, errors.New(alreadyClosedError)
	}
	return msg, nil
}

func (c testPulsarConsumer) Chan() <-chan pulsar.ConsumerMessage {
	return c.messageChan
}

func (c testPulsarConsumer) Ack(_ pulsar.Message) {
	// no-op
}

func (c testPulsarConsumer) ReconsumeLater(_ pulsar.Message, _ time.Duration) {
	panic("implement me")
}

func (c testPulsarConsumer) AckID(_ pulsar.MessageID) {
	panic("implement me")
}

func (c testPulsarConsumer) Nack(_ pulsar.Message) {
	// no-op
}

func (c testPulsarConsumer) NackID(_ pulsar.MessageID) {
	panic("implement me")
}

func (c testPulsarConsumer) Close() {}

func (c testPulsarConsumer) Seek(_ pulsar.MessageID) error {
	panic("implement me")
}

func (c testPulsarConsumer) SeekByTime(_ time.Time) error {
	panic("implement me")
}

func (c testPulsarConsumer) Name() string {
	panic("implement me")
}

type testPulsarMessage struct {
	payload []byte
}

func (msg testPulsarMessage) Topic() string {
	panic("implement me")
}

func (msg testPulsarMessage) Properties() map[string]string {
	panic("implement me")
}

func (msg testPulsarMessage) Payload() []byte {
	return msg.payload
}

func (msg testPulsarMessage) ID() pulsar.MessageID {
	panic("implement me")
}

func (msg testPulsarMessage) PublishTime() time.Time {
	panic("implement me")
}

func (msg testPulsarMessage) EventTime() time.Time {
	panic("implement me")
}

func (msg testPulsarMessage) Key() string {
	panic("implement me")
}

func (msg testPulsarMessage) OrderingKey() string {
	panic("implement me")
}

func (msg testPulsarMessage) RedeliveryCount() uint32 {
	panic("implement me")
}

func (msg testPulsarMessage) IsReplicated() bool {
	panic("implement me")
}

func (msg testPulsarMessage) GetReplicatedFrom() string {
	panic("implement me")
}

func (msg testPulsarMessage) GetSchemaValue(_ interface{}) error {
	panic("implement me")
}

func (msg testPulsarMessage) ProducerName() string {
	panic("implement me")
}

func (msg testPulsarMessage) GetEncryptionContext() *pulsar.EncryptionContext {
	panic("implement me")
}

func Test_NewLogsReceiver_Text(t *testing.T) {
	tests := []struct {
		name string
		enc  string
		text string
	}{
		{
			name: "unmarshal test for Englist (ASCII characters) with text_utf8",
			text: "ASCII characters test",
			enc:  "utf8",
		},
		{
			name: "unmarshal test for unicode with text_utf8",
			text: "UTF8 测试 測試 テスト 테스트 ☺️",
			enc:  "utf8",
		},
		{
			name: "unmarshal test for Simplified Chinese with text_gbk",
			text: "GBK 简体中文解码测试",
			enc:  "gbk",
		},
		{
			name: "unmarshal test for Japanese with text_shift_jis",
			text: "Shift_JIS 日本のデコードテスト",
			enc:  "shift_jis",
		},
		{
			name: "unmarshal test for Korean with text_euc-kr",
			text: "EUC-KR 한국 디코딩 테스트",
			enc:  "euc-kr",
		},
	}
	for _, test := range tests {
		sink := &consumertest.LogsSink{}
		ctx, cancel := context.WithCancel(context.Background())
		messageChan := make(chan pulsar.ConsumerMessage, 10)
		pulsarConsumer := testPulsarConsumer{
			messageChan: messageChan,
		}
		unmarshaler, err := newTextLogsUnmarshaler().WithEnc(test.enc)
		require.NoError(t, err)
		consumer := &pulsarLogsConsumer{
			logsConsumer: sink,
			consumer:     pulsarConsumer,
			cancel:       cancel,
			settings:     receivertest.NewNopCreateSettings(),
			unmarshaler:  unmarshaler,
			topic:        "test",
		}
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			assert.ErrorContains(t, consumeLogsLoop(ctx, consumer), alreadyClosedError)
			wg.Done()
		}()
		encCfg := textutils.NewEncodingConfig()
		encCfg.Encoding = test.enc
		enc, err := encCfg.Build()
		require.NoError(t, err)
		encoder := enc.Encoding.NewEncoder()
		encoded, err := encoder.Bytes([]byte(test.text))
		require.NoError(t, err)
		messageChan <- pulsar.ConsumerMessage{
			Message: testPulsarMessage{
				payload: encoded,
			},
		}
		close(messageChan)
		wg.Wait()
		require.Equal(t, sink.LogRecordCount(), 1)
		log := sink.AllLogs()[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
		assert.Equal(t, log.Body().Str(), test.text)
		assert.LessOrEqual(t, log.ObservedTimestamp().AsTime(), time.Now())
	}
}

func Test_NewLogsReceiver_JSON(t *testing.T) {
	sink := &consumertest.LogsSink{}
	ctx, cancel := context.WithCancel(context.Background())
	messageChan := make(chan pulsar.ConsumerMessage, 10)
	pulsarConsumer := testPulsarConsumer{
		messageChan: messageChan,
	}
	unmarshaler := newJSONLogsUnmarshaler()
	consumer := &pulsarLogsConsumer{
		logsConsumer: sink,
		consumer:     pulsarConsumer,
		cancel:       cancel,
		settings:     receivertest.NewNopCreateSettings(),
		unmarshaler:  unmarshaler,
		topic:        "test",
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		err := consumeLogsLoop(ctx, consumer)
		assert.ErrorContains(t, err, alreadyClosedError)
		wg.Done()
	}()
	jsonStr := `{"key":"value"}`
	messageChan <- pulsar.ConsumerMessage{
		Message: testPulsarMessage{
			payload: []byte(jsonStr),
		},
	}
	close(messageChan)
	wg.Wait()
	require.Equal(t, sink.LogRecordCount(), 1)
	log := sink.AllLogs()[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	val, _ := log.Body().Map().Get("key")
	assert.Equal(t, val.Str(), "value")
	assert.Equal(t, log.Body().AsString(), jsonStr)
	assert.LessOrEqual(t, log.ObservedTimestamp().AsTime(), time.Now())
}

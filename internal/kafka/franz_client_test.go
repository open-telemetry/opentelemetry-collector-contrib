// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafka

import (
	"context"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kfake"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configtls"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka/kafkatest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/kafka/configkafka"
)

func TestNewFranzSyncProducer_SASL(t *testing.T) {
	_, clientConfig := kafkatest.NewCluster(t, kfake.EnableSASL(),
		kfake.Superuser(PLAIN, "plain_user", "plain_password"),
		kfake.Superuser(SCRAMSHA256, "scramsha256_user", "scramsha256_password"),
		kfake.Superuser(SCRAMSHA512, "scramsha512_user", "scramsha512_password"),
	)
	tryConnect := func(mechanism, username, password string) error {
		clientConfig := clientConfig // Copy the client config to avoid modifying the original
		clientConfig.Authentication.SASL = &configkafka.SASLConfig{
			Mechanism: mechanism,
			Username:  username,
			Password:  password,
			Version:   1, // kfake only supports version 1
		}
		tl := zaptest.NewLogger(t, zaptest.Level(zap.WarnLevel))
		client, err := NewFranzSyncProducer(context.Background(), clientConfig,
			configkafka.NewDefaultProducerConfig(), time.Second, tl,
		)
		if err != nil {
			return err
		}
		defer client.Close()
		return client.Ping(context.Background())
	}

	type testcase struct {
		mechanism string
		username  string
		password  string
		expecErr  bool
	}

	for name, tt := range map[string]testcase{
		"PLAIN": {
			mechanism: PLAIN,
			username:  "plain_user",
			password:  "plain_password",
		},
		"SCRAM-SHA-256": {
			mechanism: SCRAMSHA256,
			username:  "scramsha256_user",
			password:  "scramsha256_password",
		},
		"SCRAM-SHA-512": {
			mechanism: SCRAMSHA512,
			username:  "scramsha512_user",
			password:  "scramsha512_password",
		},
		"invalid_PLAIN": {
			mechanism: PLAIN,
			username:  "scramsha256_user",
			password:  "scramsha256_password",
			expecErr:  true,
		},
		"invalid_SCRAM-SHA-256": {
			mechanism: SCRAMSHA256,
			username:  "scramsha512_user",
			password:  "scramsha512_password",
			expecErr:  true,
		},
		"invalid_SCRAM-SHA-512": {
			mechanism: SCRAMSHA512,
			username:  "scramsha256_user",
			password:  "scramsha256_password",
			expecErr:  true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := tryConnect(tt.mechanism, tt.username, tt.password)
			if tt.expecErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestNewFranzSyncProducer_TLS(t *testing.T) {
	// We create an httptest.Server just so we can get its TLS configuration.
	httpServer := httptest.NewTLSServer(http.NewServeMux())
	defer httpServer.Close()
	serverTLS := httpServer.TLS
	caCert := httpServer.Certificate() // self-signed

	_, clientConfig := kafkatest.NewCluster(t, kfake.TLS(serverTLS))
	tryConnect := func(cfg configtls.ClientConfig) error {
		clientConfig := clientConfig // copy
		clientConfig.TLS = &cfg
		tl := zaptest.NewLogger(t, zaptest.Level(zap.WarnLevel))
		client, err := NewFranzSyncProducer(context.Background(), clientConfig,
			configkafka.NewDefaultProducerConfig(), time.Second, tl,
		)
		if err != nil {
			return err
		}
		defer client.Close()
		return client.Ping(context.Background())
	}

	t.Run("tls_valid_ca", func(t *testing.T) {
		t.Parallel()
		tlsConfig := configtls.NewDefaultClientConfig()
		tlsConfig.CAPem = configopaque.String(
			pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caCert.Raw}),
		)
		assert.NoError(t, tryConnect(tlsConfig))
	})

	t.Run("tls_insecure_skip_verify", func(t *testing.T) {
		t.Parallel()
		tlsConfig := configtls.NewDefaultClientConfig()
		tlsConfig.InsecureSkipVerify = true
		require.NoError(t, tryConnect(tlsConfig))
	})

	t.Run("tls_unknown_ca", func(t *testing.T) {
		t.Parallel()
		config := configtls.NewDefaultClientConfig()
		err := tryConnect(config)
		require.Error(t, err)
		assert.ErrorContains(t, err, "x509: certificate signed by unknown authority")
	})

	t.Run("plaintext", func(t *testing.T) {
		t.Parallel()
		// Should fail because the server expects TLS.
		require.Error(t, tryConnect(configtls.ClientConfig{}))
	})
}

func TestNewFranzSyncProducerCompression(t *testing.T) {
	compressionAlgos := []string{"none", "gzip", "snappy", "lz4", "zstd"}
	for i, compressionAlgo := range compressionAlgos {
		t.Run(compressionAlgo, func(t *testing.T) {
			t.Parallel()
			validTopic := fmt.Sprintf("test-topic-%s", compressionAlgo)
			cluster, clientConfig := kafkatest.NewCluster(t,
				kfake.SeedTopics(1, validTopic),
			)
			prodCfg := configkafka.NewDefaultProducerConfig()
			prodCfg.Compression = compressionAlgo

			tl := zaptest.NewLogger(t, zaptest.Level(zap.InfoLevel))
			client, err := NewFranzSyncProducer(context.Background(), clientConfig, prodCfg, time.Second, tl)
			require.NoError(t, err)
			defer client.Close()

			ctx, cancel := context.WithTimeoutCause(context.Background(), time.Second,
				errors.New("Failed to connect to Kafka cluster"),
			)
			defer cancel()
			require.NoError(t, client.Ping(ctx))

			// Franz-go only send the compressed message if the compressed size
			// is smaller than the uncompressed size.
			body := "test message: This is a sample message used for testing Kafka compression algorithms. The content is intentionally long to ensure that compression is applied and can be verified during the test. The message contains a variety of words and phrases to simulate realistic payloads that might be encountered in production environments. This helps ensure that the compression logic works as expected and that the system can handle typical workloads safely and efficiently."
			cluster.ControlKey(int16(kmsg.Produce), func(kreq kmsg.Request) (kmsg.Response, error, bool) {
				preq := kreq.(*kmsg.ProduceRequest)
				assert.Len(t, preq.Topics, 1, "expected one topic in produce request")
				assert.Equal(t, validTopic, preq.Topics[0].Topic, "produced to wrong topic")
				assert.Len(t, preq.Topics[0].Partitions, 1, "expected one partition in produce request")
				assert.Equal(t, int32(0), preq.Topics[0].Partitions[0].Partition, "produced to wrong partition")

				var rb kmsg.RecordBatch
				require.NoError(t, rb.ReadFrom(preq.Topics[0].Partitions[0].Records))
				// Check the compression bits (lowest 3 bits)
				compressionBits := rb.Attributes & 0x07
				assert.Equal(t, int16(i), compressionBits)
				assert.Equal(t, int32(1), rb.NumRecords, "expected one record in produce request")

				return &kmsg.ProduceResponse{
					Version: kreq.GetVersion(),
					Topics: []kmsg.ProduceResponseTopic{{
						Topic: validTopic,
						Partitions: []kmsg.ProduceResponseTopicPartition{
							kmsg.NewProduceResponseTopicPartition(),
						},
					}},
				}, nil, true
			})

			result := client.ProduceSync(ctx, &kgo.Record{
				Topic: validTopic, Value: []byte(body),
			})
			client.Close()
			require.Len(t, result, 1, "expected one produce result")
			res := result[0]
			require.NoError(t, res.Err, "failed to produce message: %v", res.Err)
			assert.Equal(t, validTopic, res.Record.Topic, "produced message to wrong topic")
			assert.Equal(t, uint8(i), res.Record.Attrs.CompressionType())
			assert.Equal(t, []byte(body), res.Record.Value, "produced message with wrong value")
		})
	}
}

func TestNewFranzSyncProducerRequiredAcks(t *testing.T) {
	topic := "topic"
	_, clientConfig := kafkatest.NewCluster(t, kfake.SeedTopics(1, topic))
	acks := []configkafka.RequiredAcks{
		configkafka.NoResponse,
		configkafka.WaitForLocal,
		configkafka.WaitForAll,
	}
	for _, ack := range acks {
		t.Run(acksToString(t, ack), func(t *testing.T) {
			t.Parallel()
			prodCfg := configkafka.NewDefaultProducerConfig()
			prodCfg.RequiredAcks = ack

			tl := zaptest.NewLogger(t, zaptest.Level(zap.WarnLevel))
			client, err := NewFranzSyncProducer(context.Background(), clientConfig, prodCfg, time.Second, tl)
			require.NoError(t, err)
			defer client.Close()

			ctx, cancel := context.WithTimeoutCause(context.Background(), time.Second,
				errors.New("Failed to connect to Kafka cluster"),
			)
			defer cancel()
			require.NoError(t, client.Ping(ctx))

			// Produce a test message.
			result := client.ProduceSync(ctx, &kgo.Record{
				Topic: topic, Value: []byte("test message"),
			})
			for _, v := range result {
				require.NoError(t, v.Err, "failed to produce message: %v", v.Err)
				assert.Equal(t, topic, v.Record.Topic, "produced message to wrong topic")
			}
		})
	}
}

func acksToString(tb testing.TB, acks configkafka.RequiredAcks) string {
	switch acks {
	case configkafka.NoResponse:
		return "NoResponse"
	case configkafka.WaitForLocal:
		return "WaitForLocal"
	case configkafka.WaitForAll:
		return "WaitForAll"
	default:
		tb.Fatalf("unknown RequiredAcks value: %v", acks)
		return "" // Unreachable, but required.
	}
}

// TODO(marclop): Remove this test once we completely remove Sarama so
// we can get rid of the sarama dependency.
func Test_saramaCompatHasher(t *testing.T) {
	cases := []struct {
		name       string
		key        []byte
		topic      string
		partitions int32
	}{
		{"empty topic", []byte("key1"), "", 3},
		{"single partition", []byte("key2"), "topic2", 1},
		{"large partitions", []byte("key3"), "topic3", 100},
		{"unicode key", []byte("ключ"), "topic4", 5},
		{"unicode topic", []byte("key5"), "тема", 4},
		{"zero partitions", []byte("key6"), "topic6", 1},
		{"long key", []byte("thisisaverylongkeythatexceedstypicallengths"), "topic7", 8},
		{"long topic", []byte("key8"), "averylongtopicnamethatexceedstypicallengths", 10},
		{"special chars key", []byte("!@#$%^&*()_+"), "topic9", 7},
		{"case sensitivity", []byte("Key11"), "Topic11", 11},
		{"case sensitivity 2", []byte("key11"), "topic11", 11},
		{"max int32 partitions", []byte("key12"), "topic12", 2147483647},
		// Original cases for coverage
		{"orig case 1", []byte("key1"), "topic1", 3},
		{"orig case 2", []byte("key2"), "topic2", 5},
		{"orig case 3", []byte("key3"), "topic3", 7},
		{"orig case 4", []byte("key4"), "topic4", 2},
		{"orig case 5", []byte("key5"), "topic5", 4},
		{"orig case 6", []byte("key6"), "topic6", 6},
		{"orig case 7", []byte("key7"), "topic7", 8},
		{"orig case 8", []byte("key8"), "topic8", 10},
		{"orig case 9", []byte("key9"), "topic9", 1},
		{"orig case 10", []byte("key10"), "topic10", 9},
		{"orig case 11", []byte("key11"), "topic11", 11},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			message := "test message"
			// Sarama result
			r, err := sarama.NewHashPartitioner(tc.topic).Partition(&sarama.ProducerMessage{
				Topic: tc.topic,
				Key:   sarama.ByteEncoder(tc.key),
				Value: sarama.ByteEncoder(message),
			}, tc.partitions)
			require.NoError(t, err, "failed to hash partition")
			saramaResult := int(r)

			// Franz-go result
			franzResult := newSaramaCompatPartitioner().ForTopic(tc.topic).Partition(&kgo.Record{
				Topic: tc.topic,
				Key:   tc.key,
				Value: []byte(message),
			}, int(tc.partitions))
			assert.Equal(t, saramaResult, franzResult, "partitioning results do not match")
		})
	}
}

func TestNewFranzKafkaConsumerRegex(t *testing.T) {
	topicCount := 10
	topics := make([]string, topicCount)
	topicPrefix := "topic-"
	for i := 0; i < topicCount; i++ {
		topics[i] = fmt.Sprintf("%s%d", topicPrefix, i)
	}
	_, clientConfig := kafkatest.NewCluster(t, kfake.SeedTopics(1, topics...))
	regexTopic := []string{"^" + topicPrefix + ".*"}
	consumeConfig := configkafka.NewDefaultConsumerConfig()
	// Set to earliest commit so we don't have to worry about synchronizing the
	// producer and consumer.
	consumeConfig.InitialOffset = configkafka.EarliestOffset

	client := mustNewFranzConsumerGroup(t, clientConfig, consumeConfig, regexTopic)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	recordChan := fetchRecords(ctx, client, topicCount)
	recordValue := []byte("test message")
	rs := make([]*kgo.Record, 0, topicCount)
	for _, topic := range topics {
		rs = append(rs, &kgo.Record{Topic: topic, Value: recordValue})
	}
	client.ProduceSync(ctx, rs...)

	fetch := <-recordChan
	require.NoError(t, fetch.Err())
	assert.Equal(t, topicCount, fetch.NumRecords())
	seenTopics := make([]string, 0, topicCount)
	fetch.EachRecord(func(r *kgo.Record) {
		assert.Contains(t, r.Topic, topicPrefix)
		assert.Len(t, r.Topic, len(topicPrefix)+1)
		assert.Equal(t, recordValue, r.Value)
		seenTopics = append(seenTopics, r.Topic)
	})
	sort.Strings(seenTopics)
	assert.Equal(t, seenTopics, topics)
}

type onBrokerWrite func(meta kgo.BrokerMetadata, key int16, bytesWritten int, writeWait, timeToWrite time.Duration, err error)

func (f onBrokerWrite) OnBrokerWrite(meta kgo.BrokerMetadata, key int16, bytesWritten int, writeWait, timeToWrite time.Duration, err error) {
	f(meta, key, bytesWritten, writeWait, timeToWrite, err)
}

func TestNewFranzKafkaConsumer_InitialOffset(t *testing.T) {
	for _, initial := range []string{configkafka.EarliestOffset, configkafka.LatestOffset} {
		t.Run(initial, func(t *testing.T) {
			topic := "topic"
			_, clientConfig := kafkatest.NewCluster(t, kfake.SeedTopics(1, topic))
			consumeConfig := configkafka.NewDefaultConsumerConfig()
			consumeConfig.InitialOffset = initial
			fetchIssued := make(chan struct{})
			var once2 sync.Once
			client := mustNewFranzConsumerGroup(t, clientConfig, consumeConfig, []string{topic},
				kgo.WithHooks(onBrokerWrite(func(_ kgo.BrokerMetadata, key int16, _ int, _, _ time.Duration, _ error) {
					if key == kmsg.Fetch.Int16() {
						once2.Do(func() { close(fetchIssued) })
					}
				})),
			)

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			produce := func() {
				require.NoError(t, client.ProduceSync(ctx, &kgo.Record{
					Topic: topic, Value: []byte("test message"),
				}).FirstErr())
			}
			produce() // Produce before consuming

			// Depending on the Initial offset configuration, the consumer will
			// fetch 1 or 2 records.
			var expected int
			switch initial {
			case configkafka.EarliestOffset:
				expected = 2
			case configkafka.LatestOffset:
				expected = 1
			}
			recordChan := fetchRecords(ctx, client, expected)

			// Wait until the consumer issues the fetch request to produce.
			select {
			case <-fetchIssued:
			case <-ctx.Done():
				t.Fatalf("timeout waiting for the partition to be assigned")
			}
			produce() // Produce again.

			fetch := <-recordChan
			require.NoError(t, fetch.Err())
			assert.Equal(t, expected, fetch.NumRecords())
			fetch.EachRecord(func(r *kgo.Record) {
				assert.Equal(t, []byte("test message"), r.Value)
			})
		})
	}
}

func fetchRecords(ctx context.Context, client *kgo.Client, wantRecords int) <-chan kgo.Fetches {
	fetchChan := make(chan kgo.Fetches)
	go func() {
		var records int
		var fetches kgo.Fetches
		defer func() {
			fetchChan <- fetches
			close(fetchChan)
		}()
		for {
			select {
			case <-ctx.Done():
				return
			default:
				fetch := client.PollRecords(ctx, wantRecords)
				records += fetch.NumRecords()
				fetches = append(fetches, fetch...)
				if records == wantRecords {
					return
				}
			}
		}
	}()
	return fetchChan
}

func mustNewFranzConsumerGroup(t *testing.T,
	clientConfig configkafka.ClientConfig,
	consumerConfig configkafka.ConsumerConfig,
	topics []string, opts ...kgo.Opt,
) *kgo.Client {
	t.Helper()
	// We want to keep the metadata cache very short lived in tests to speed
	// up and avoid waiting for too long.
	minAge := 10 * time.Millisecond
	opts = append(opts, kgo.MetadataMinAge(minAge), kgo.MetadataMaxAge(minAge*2))
	client, err := NewFranzConsumerGroup(context.Background(), clientConfig, consumerConfig,
		topics, zaptest.NewLogger(t, zaptest.Level(zap.InfoLevel)), opts...,
	)
	require.NoError(t, err)
	t.Cleanup(client.Close)
	return client
}

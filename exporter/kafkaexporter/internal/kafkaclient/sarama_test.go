// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaclient

import (
	"errors"
	"testing"

	"github.com/IBM/sarama"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/consumererror"
)

func TestWrapKafkaProducerError(t *testing.T) {
	t.Run("should return permanent error on configuration error", func(t *testing.T) {
		err := sarama.ConfigurationError("configuration error")
		prodErrs := sarama.ProducerErrors{
			&sarama.ProducerError{Err: err},
		}

		got := wrapKafkaProducerError(prodErrs)

		assert.True(t, consumererror.IsPermanent(got))
		assert.Contains(t, got.Error(), err.Error())
	})

	t.Run("should return permanent error whne multiple configuration error", func(t *testing.T) {
		err := sarama.ConfigurationError("configuration error")
		prodErrs := sarama.ProducerErrors{
			&sarama.ProducerError{Err: err},
			&sarama.ProducerError{Err: err},
		}

		got := wrapKafkaProducerError(prodErrs)

		assert.True(t, consumererror.IsPermanent(got))
		assert.Contains(t, got.Error(), err.Error())
	})

	t.Run("should return not permanent error when at least one not configuration error", func(t *testing.T) {
		err := sarama.ConfigurationError("configuration error")
		prodErrs := sarama.ProducerErrors{
			&sarama.ProducerError{Err: err},
			&sarama.ProducerError{Err: errors.New("other producer error")},
		}

		got := wrapKafkaProducerError(prodErrs)

		assert.False(t, consumererror.IsPermanent(got))
		assert.Contains(t, got.Error(), err.Error())
	})

	t.Run("should return not permanent error on other producer error", func(t *testing.T) {
		err := errors.New("other producer error")
		prodErrs := sarama.ProducerErrors{
			&sarama.ProducerError{Err: err},
		}

		got := wrapKafkaProducerError(prodErrs)

		assert.False(t, consumererror.IsPermanent(got))
		assert.Contains(t, got.Error(), err.Error())
	})

	t.Run("should return not permanent error when other error", func(t *testing.T) {
		err := errors.New("other error")

		got := wrapKafkaProducerError(err)

		assert.False(t, consumererror.IsPermanent(got))
		assert.Contains(t, got.Error(), err.Error())
	})
}

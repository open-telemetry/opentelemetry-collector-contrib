package remotetapprocessor

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"golang.org/x/time/rate"

	"go.opentelemetry.io/collector/processor"
)

func TestWriteToChannelSet(t *testing.T) {
	wg := &sync.WaitGroup{}
	cases := []struct {
		name  string
		limit int
	}{
		{name: "1", limit: 1},
		{name: "1", limit: 10},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := newProcessor(processor.CreateSettings{}, &Config{Limit: rate.Limit(c.limit)})
			ch := make(chan []byte)
			idx := p.cs.add(ch)

			ctx, _ := context.WithTimeout(context.Background(), time.Second*10)

			wg.Add(1)
			// receive message func
			go func() {
				defer wg.Done()

				receiveMap := make(map[int64]int)
				for bytes := range ch {
					assert.Equal(t, bytes, []byte("hello opentelemetry"))
					receiveMap[time.Now().Unix()] = receiveMap[time.Now().Unix()] + 1
				}

				expectNum := 0
				for _, v := range receiveMap {
					if v == c.limit {
						expectNum++
					}
				}

				assert.True(t, float64(expectNum)/float64(len(receiveMap)) > 0.8)
			}()

			wg.Add(1)
			// send message func
			go func() {
				defer wg.Done()

				for {
					select {
					case <-ctx.Done():
						p.cs.closeAndRemove(idx)
						return
					default:
						p.writeToChannelSet([]byte("hello opentelemetry"))
					}
				}
			}()

			wg.Wait()
		})
	}
}

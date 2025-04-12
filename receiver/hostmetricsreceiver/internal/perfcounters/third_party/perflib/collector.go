//go:build windows

package perflib // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/perfcounters/third_party/perflib"

import (
	"fmt"
	"reflect"
	"strings"
)

type Collector struct {
	object string
	query  string

	counters       map[string]Counter
	nameIndexValue int
}

type Counter struct {
	Name      string
	Desc      string
	Instances map[string]uint32
	Type      uint32
	Frequency float64

	FieldIndexValue       int
	FieldIndexSecondValue int
}

func NewCollector[T any](object string, _ []string) (*Collector, error) {
	collector := &Collector{
		object:         object,
		query:          MapCounterToIndex(object),
		nameIndexValue: -1,
		counters:       make(map[string]Counter),
	}

	var values [0]T
	valueType := reflect.TypeOf(values).Elem()

	if f, ok := valueType.FieldByName("Name"); ok {
		if f.Type.Kind() == reflect.String {
			collector.nameIndexValue = f.Index[0]
		}
	}

	for _, f := range reflect.VisibleFields(valueType) {
		counterName, ok := f.Tag.Lookup("perfdata")
		if !ok {
			continue
		}

		var counter Counter
		if counter, ok = collector.counters[counterName]; !ok {
			counter = Counter{
				Name:                  counterName,
				FieldIndexSecondValue: -1,
				FieldIndexValue:       -1,
			}
		}

		if strings.HasSuffix(counterName, ",secondvalue") {
			counterName = strings.TrimSuffix(counterName, ",secondvalue")

			counter.FieldIndexSecondValue = f.Index[0]
		} else {
			counter.FieldIndexValue = f.Index[0]
		}

		collector.counters[counterName] = counter
	}

	var collectValues []T

	if err := collector.Collect(&collectValues); err != nil {
		return nil, fmt.Errorf("failed to collect initial data: %w", err)
	}

	return collector, nil
}

func (c *Collector) Describe() map[string]string {
	return map[string]string{}
}

func (c *Collector) Collect(data any) error {
	dv := reflect.ValueOf(data)
	if dv.Kind() != reflect.Ptr || dv.IsNil() {
		return ErrInvalidEntityType
	}

	dv = dv.Elem()

	elemType := dv.Type().Elem()
	elemValue := reflect.ValueOf(reflect.New(elemType).Interface()).Elem()

	if dv.Kind() != reflect.Slice || elemType.Kind() != reflect.Struct {
		return ErrInvalidEntityType
	}

	perfObjects, err := QueryPerformanceData(c.query, c.object)
	if err != nil {
		return fmt.Errorf("QueryPerformanceData: %w", err)
	}

	if len(perfObjects) == 0 || perfObjects[0] == nil || len(perfObjects[0].Instances) == 0 {
		return nil
	}

	if dv.Len() != 0 {
		dv.Set(reflect.MakeSlice(dv.Type(), 0, len(perfObjects[0].Instances)))
	}

	dv.Clear()

	for _, perfObject := range perfObjects {
		if perfObject.Name != c.object {
			continue
		}

		for _, perfInstance := range perfObject.Instances {
			instanceName := perfInstance.Name
			if strings.HasSuffix(instanceName, "_Total") {
				continue
			}

			if instanceName == "" || instanceName == "*" {
				instanceName = InstanceEmpty
			}

			if c.nameIndexValue != -1 {
				elemValue.Field(c.nameIndexValue).SetString(instanceName)
			}

			dv.Set(reflect.Append(dv, elemValue))
			index := dv.Len() - 1

			for _, perfCounter := range perfInstance.Counters {
				if perfCounter.Def.IsBaseValue && !perfCounter.Def.IsNanosecondCounter {
					continue
				}

				counter, ok := c.counters[perfCounter.Def.Name]
				if !ok {
					continue
				}

				switch perfCounter.Def.CounterType {
				case PERF_ELAPSED_TIME:
					dv.Index(index).
						Field(counter.FieldIndexValue).
						SetFloat(float64((perfCounter.Value - WindowsEpoch) / perfObject.Frequency))
				case PERF_100NSEC_TIMER, PERF_PRECISION_100NS_TIMER:
					dv.Index(index).
						Field(counter.FieldIndexValue).
						SetFloat(float64(perfCounter.Value) * TicksToSecondScaleFactor)
				default:
					if counter.FieldIndexSecondValue != -1 {
						dv.Index(index).
							Field(counter.FieldIndexSecondValue).
							SetFloat(float64(perfCounter.SecondValue))
					}

					if counter.FieldIndexValue != -1 {
						dv.Index(index).
							Field(counter.FieldIndexValue).
							SetFloat(float64(perfCounter.Value))
					}
				}
			}
		}
	}

	return nil
}

func (c *Collector) Close() {}

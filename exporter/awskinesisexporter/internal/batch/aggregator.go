package batch

import (
	"crypto/md5"

	k "github.com/aws/aws-sdk-go/service/kinesis"
	"google.golang.org/protobuf/proto"
)

var (
	magicNumber           = []byte{0xF3, 0x89, 0x9A, 0xC2}
	partitionKeyIndexSize = 8
	maxAggregationCount   = 4294967295
	maxRecordSize         = 1 << 20
)

type Aggregator struct {
	buf    []*Record
	pkeys  []string
	nbytes int
}

func (a *Aggregator) IsRecordAggregative(data []byte, partitionKey string) bool {
	return data != nil && len(data)+len([]byte(partitionKey)) <= maxRecordSize
}

func (a *Aggregator) IsBatchFull(data []byte, partitionKey string) bool {
	nbytes := len(data) + len([]byte(partitionKey))
	return nbytes+a.nbytes+md5.Size+len(magicNumber)+partitionKeyIndexSize > maxRecordSize || len(a.buf) >= maxAggregationCount
}

func (a *Aggregator) Put(data []byte, partitionKey string) {
	if len(a.pkeys) == 0 {
		a.pkeys = []string{partitionKey}
		a.nbytes += len([]byte(partitionKey))
	}
	keyIndex := uint64(len(a.pkeys) - 1)

	a.nbytes += partitionKeyIndexSize
	a.buf = append(a.buf, &Record{
		Data:              data,
		PartitionKeyIndex: &keyIndex,
	})
	a.nbytes += len(data)
}

func (a *Aggregator) Drain() (*k.PutRecordsRequestEntry, error) {
	if a.nbytes == 0 {
		return nil, nil
	}
	data, err := proto.Marshal(&AggregatedRecord{
		PartitionKeyTable: a.pkeys,
		Records:           a.buf,
	})
	if err != nil {
		return nil, err
	}
	h := md5.New()
	h.Write(data)
	checkSum := h.Sum(nil)
	aggData := append(magicNumber, data...)
	aggData = append(aggData, checkSum...)
	entry := &k.PutRecordsRequestEntry{
		Data:         aggData,
		PartitionKey: &a.pkeys[0],
	}
	a.clear()
	return entry, nil
}

func (a *Aggregator) clear() {
	a.buf = make([]*Record, 0)
	a.pkeys = make([]string, 0)
	a.nbytes = 0
}

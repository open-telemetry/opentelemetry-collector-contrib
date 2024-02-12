package ackextension

type AckExtension interface {
	// ProcessEvent marks the beginning of processing an event. It generates an ack ID for the associated partition ID.
	// ACK IDs are only unique within a partition. Two partitions can have the same ACK IDs but they are generated for different events.
	ProcessEvent(partitionID string) (ackID uint64)

	// Ack acknowledges an event has been processed.
	Ack(partitionID string, ackID uint64)

	// QueryAcks checks the statuses of given ackIDs for a partition.
	QueryAcks(partitionID string, ackIDs []uint64) map[uint64]bool
}

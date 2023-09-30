package fileconsumer

import "context"

// following methods are only applicable to threadpool
func (m *Manager) worker(ctx context.Context, queue chan readerEnvelope) {
	// empty for now, will expand later
}

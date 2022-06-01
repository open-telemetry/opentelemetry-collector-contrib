package schema

// Schema abstracts the parsed translation file
// into a useable
type Schema interface {
	SupportedVersion(version *Version) bool
}

// NoopSchema is used when the schemaURL is not valid
// to help reduce the amount of branching that would be
// needed in the event that nil was returned in its place
type NoopSchema struct{}

var _ Schema = (*NoopSchema)(nil)

func (NoopSchema) SupportedVersion(_ *Version) bool { return false }

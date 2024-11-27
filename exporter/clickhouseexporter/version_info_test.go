package clickhouseexporter

// TestCollectorVersionResolver will return a constant value for the collector version.
type TestCollectorVersionResolver struct {
	version string
}

func NewTestCollectorVersionResolver(version string) *TestCollectorVersionResolver {
	return &TestCollectorVersionResolver{version: version}
}

func NewDefaultTestCollectorVersionResolver() *TestCollectorVersionResolver {
	return &TestCollectorVersionResolver{version: "test"}
}

func (r *TestCollectorVersionResolver) GetVersion() string {
	return r.version
}

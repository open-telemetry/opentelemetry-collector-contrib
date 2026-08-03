package configfile

// FileEntry is one configured path to watch. Format is optional (auto-detected from extension when empty).
type FileEntry struct {
	Path   string `mapstructure:"path"`
	Format string `mapstructure:"format"` // auto|generic|ini|yaml|json
}

// PollerConfig groups runtime settings for the configfile receiver poller.
type PollerConfig struct {
	Files              []FileEntry
	ExcludeKeys        []string
	MaxKeysPerFile     int
	PollIntervalSecond int
	StatePath          string
}

func (c PollerConfig) options() Options {
	return Options{
		ExcludeKeys:    c.ExcludeKeys,
		MaxKeysPerFile: c.MaxKeysPerFile,
	}
}

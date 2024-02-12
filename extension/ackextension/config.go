package ackextension

type Config struct {
	// StorageType defines the storage type of the extension. Currently planned for In memory type. Future consideration is disk type.
	StorageType string `mapstructure:"StorageType,omitempty"`
}

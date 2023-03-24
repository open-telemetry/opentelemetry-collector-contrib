package apachepulsarreceiver

type client interface {
	GetTenants()
	GetNameSpaces(tenant string)
	GetTopics()
	GetTopicStats()
	Connect() error
	Close() error
}

func GetTenants() {
	// tenants.List() returns a slice of strings and an error
}

func GetNameSpaces(tenant string) {
	// GetNamespaces(tenant string) returns a list of all namespaces for a given tenant
}

func GetTopics() {
	// namespace.List(namespace string) returns a list of topics under a given namespace
}

func GetTopicStats() {
	// topic.GetStats(utils.TopicName) returns the stats for a topic
}

func Connect() {
}

func Close() {

}

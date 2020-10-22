package ecs

type ecsMetadataProvider interface {
	fetchTaskMetaData(tmde string) (*TaskMetaData, error)
	fetchContainerMetaData(tmde string) (*Container, error)
}

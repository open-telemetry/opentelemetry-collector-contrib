# Observers

Observers are implemented as an extension to discover networked endpoints like a Kubernetes pod, Docker container, or local listening port. Other components can subscribe to an observer instance to be notified of endpoints coming and going.

Currently the only component that uses observers is the [receiver_creator](../../receiver/receivercreator/README.md).

## Current Observers

* [docker_observer](dockerobserver/README.md)
* [ecs_observer](ecsobserver/README.md)
* [ecs_task_observer](ecstaskobserver/README.md)
* [host_observer](hostobserver/README.md)
* [k8s_observer](k8sobserver/README.md)

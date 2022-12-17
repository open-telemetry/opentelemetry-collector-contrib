k8seventsreceiver:
    custom static log added to show "k8s.cluster" entry

k8sclusterreceiver:
    bug-fix:
        k8s.namespace.name -- value correction 

kubeletstatsreceiver:
    new-fields:
        attribute:
            k8s.node.uid

dockerstatsreceiver:
     new-fields:
        attributes:
            container.started_on
        metric:
            container.status

hostmetricsreceiver:
    new-fields:
        attributes:
            process.started_on
        metric:
            system.disk.io.speed
            system.network.io.bandwidth
            process.memory.percent
            process.cpu.percent
    new-feature:
        avoid_selected_errors flag added -- to hide non-relevant errors

fluentforwardreciever:
    bug-fix:
        uint64 timestamp handled

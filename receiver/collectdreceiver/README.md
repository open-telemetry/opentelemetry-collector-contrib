CollectD `write_http` plugin JSON receiver

This receiver can receive data exported by the CollectD's `write_http` plugin. Only JSON format is supported. Authentication is not supported but support can be added later if needed.

This receiver was donated by SignalFx and ported from SignalFx's Gateway (https://github.com/signalfx/gateway/tree/master/protocol/collectd). As a result, this receiver supports some additional features that are technically not compatible with stock CollectD's write_http plugin. That said, in practice such incompatibilities should never surface. For example, this receiver supports extracting labels from different fields. Given a field value `field[a=b, k=v]`, this receiver will extract `a` and  `b` as label keys and, `k` and `v` as the respective label values. 

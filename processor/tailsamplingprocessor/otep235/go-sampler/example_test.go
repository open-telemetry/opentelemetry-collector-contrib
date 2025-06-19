package sampler

import (
	"fmt"

	"go.opentelemetry.io/otel/attribute"
)

func makeAF(kvs ...attribute.KeyValue) func() []attribute.KeyValue {
	return func() []attribute.KeyValue {
		return kvs
	}
}
func ExampleThreeWayParentBased() {
	root := ComposableAlwaysSample()
	local := makeAF(attribute.String("local", "true"))
	remote := makeAF(attribute.String("remote", "true"))
	sampler := RuleBased(
		WithRule(IsRootPredicate(), root),
		WithRule(IsRemotePredicate(), AnnotatingSampler(ParentThreshold(), WithSampledAttributes(remote))),
		WithDefaultRule(AnnotatingSampler(ParentThreshold(), WithSampledAttributes(local))),
	)
	fmt.Println(sampler.Description())
	// Output: RuleBased{rule(root?)=AlwaysOn,rule(remote?)=Annotate(ParentThreshold, remote=true),rule(true)=Annotate(ParentThreshold, local=true)}
}

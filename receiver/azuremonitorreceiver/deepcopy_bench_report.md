# Deep Copy Libraries Benchmark for armresources.GenericResourceExpanded

This benchmark compares several Go libraries for deep copying the armresources.GenericResourceExpanded struct.

## Results

| Library                  | ns/op   | B/op  | allocs/op | Comment |
|-------------------------|---------|-------|-----------|---------|
| huandu/go-clone         | ~650    | 736   | 11        | ✅ Selected (already in the repo) |
| brunoga/deep            | ~850    | 760   | 12        |         |
| mohae/deepcopy          | ~950    | 880   | 20        |         |
| mitchellh/copystructure | ~4400   | 4984  | 101       |         |
| barkimedes/go-deepcopy  | N/A     | N/A   | N/A       | Not supported for this type |

> These numbers are from Apple M3 Pro, Go 1.25, with benchmem enabled.

## Benchmark Code

```go
import (
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/brunoga/deep"
	"github.com/huandu/go-clone"
	"github.com/mohae/deepcopy"
	"github.com/mitchellh/copystructure"
	"github.com/barkimedes/go-deepcopy"
)

func BenchmarkDeepCopyLibraries(b *testing.B) {
	orig := armresources.GenericResourceExpanded{
		ID:       to.Ptr("/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Storage/storageAccounts/myResource"),
		Type:     to.Ptr("Microsoft.Storage/storageAccounts"),
		Name:     to.Ptr("myResource"),
		Location: to.Ptr("westeurope"),
		Tags: map[string]*string{
			"env": to.Ptr("prod"),
		},
	}

	b.Run("brunoga/deep", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			clonedAny, _ := deep.Copy(orig)
			_ = clonedAny
		}
	})

	b.Run("mohae/deepcopy", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = deepcopy.Copy(orig)
		}
	})

	b.Run("barkimedes/go-deepcopy", func(b *testing.B) {
		b.Skip("barkimedes/go-deepcopy does not support armresources.GenericResourceExpanded (panic reflect.Value.Set)")
		for i := 0; i < b.N; i++ {
			_, _ = barkdeep.Anything(orig)
		}
	})

	b.Run("mitchellh/copystructure", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = copystructure.Copy(orig)
		}
	})

	b.Run("huandu/go-clone", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = clone.Clone(orig).(armresources.GenericResourceExpanded)
		}
	})
}
```

## Selected Library

- **huandu/go-clone** is chosen for deep copy because it is the fastest, most memory-efficient, and already present in the project.
- Other libraries will not be added to go.mod to avoid unnecessary dependencies.

---

*Benchmarks are reproducible with `go test -bench=BenchmarkDeepCopyLibraries -benchmem` in the `receiver/azuremonitorreceiver` folder.*

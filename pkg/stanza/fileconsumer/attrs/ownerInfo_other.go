//go:build !linux && !solaris

package attrs

import (
	"fmt"
	"os"
)

func (r *Resolver) addOwnerInfo(file *os.File, attributes map[string]any) (err error) {
	return fmt.Errorf("addOwnerInfo it's only implemented for linux or solaris: %w", err)
}

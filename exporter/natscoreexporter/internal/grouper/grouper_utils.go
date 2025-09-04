package grouper

import (
	"fmt"
	"strings"
)

func validateSubject(subject string) error {
	for _, c := range []rune{'\x00', ' ', '*', '>'} {
		if strings.ContainsRune(subject, c) {
			return fmt.Errorf("subject contains forbidden character: %c", c)
		}
	}
	return nil
}

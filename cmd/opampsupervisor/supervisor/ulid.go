package supervisor

import (
	"crypto/rand"
	"os"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
)

func loadULIDFromFile(file string) (ulid.ULID, error) {
	by, err := os.ReadFile(file)
	if err != nil {
		return ulid.ULID{}, err
	}

	ulidAsString := string(by)
	return ulid.Parse(strings.TrimSpace(ulidAsString))
}

func saveULIDToFile(file string, id ulid.ULID) error {
	ulidAsString := id.String()
	return os.WriteFile(file, []byte(ulidAsString), 0600)
}

func generateNewULID() (ulid.ULID, error) {
	entropy := ulid.Monotonic(rand.Reader, 0)
	id, err := ulid.New(ulid.Timestamp(time.Now()), entropy)
	if err != nil {
		return ulid.ULID{}, err
	}

	return id, nil
}

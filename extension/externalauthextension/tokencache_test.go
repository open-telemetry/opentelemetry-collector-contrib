package externalauthextension

import (
	"testing"
	"time"
)

func TestValidateTokenForNonExistentToken(t *testing.T) {
	tc := newTokenCache()
	token := "nonExistentToken"

	if tc.validateToken(token) {
		t.Errorf("Expected validation to fail for non-existent token")
	}
}
func TestIsTokenValidForNonExistentToken(t *testing.T) {
	tc := newTokenCache()
	token := "nonExistentToken"

	if tc.isTokenValid(token) {
		t.Errorf("Expected non-existent token to be invalid")
	}
}

func TestIsTokenExpiredForNonExistentToken(t *testing.T) {
	tc := newTokenCache()
	token := "nonExistentToken"
	duration := "1s"

	if !tc.isTokenExpired(token, duration) {
		t.Errorf("Expected non-existent token to be expired")
	}
}

func TestAddToken(t *testing.T) {
	tc := newTokenCache()
	token := "testToken"
	valid := true

	tc.addToken(token, valid)

	if !tc.tokenExists(token) {
		t.Errorf("Expected token to exist in cache")
	}

	if !tc.isTokenValid(token) {
		t.Errorf("Expected token to be valid")
	}
}

func TestValidateToken(t *testing.T) {
	tc := newTokenCache()
	token := "testToken"
	valid := true

	tc.addToken(token, valid)
	if !tc.validateToken(token) {
		t.Errorf("Expected token to be validated")
	}

	if !tc.isTokenValid(token) {
		t.Errorf("Expected token to be valid after validation")
	}
}

func TestInvalidateToken(t *testing.T) {
	tc := newTokenCache()
	token := "testToken"
	valid := true

	tc.addToken(token, valid)
	tc.invalidateToken(token)

	if tc.isTokenValid(token) {
		t.Errorf("Expected token to be invalid")
	}
}

func TestIsTokenExpired(t *testing.T) {
	tc := newTokenCache()
	token := "testToken"
	valid := true
	duration := "1s"

	tc.addToken(token, valid)
	time.Sleep(2 * time.Second)

	if !tc.isTokenExpired(token, duration) {
		t.Errorf("Expected token to be expired")
	}
}

func TestTokenExists(t *testing.T) {
	tc := newTokenCache()
	token := "testToken"
	valid := true

	tc.addToken(token, valid)

	if !tc.tokenExists(token) {
		t.Errorf("Expected token to exist in cache")
	}

	nonExistentToken := "nonExistentToken"
	if tc.tokenExists(nonExistentToken) {
		t.Errorf("Expected token to not exist in cache")
	}
}

func TestIsTokenValid(t *testing.T) {
	tc := newTokenCache()
	token := "testToken"
	valid := true

	tc.addToken(token, valid)

	if !tc.isTokenValid(token) {
		t.Errorf("Expected token to be valid")
	}

	tc.invalidateToken(token)

	if tc.isTokenValid(token) {
		t.Errorf("Expected token to be invalid")
	}
}

func TestSetTokenExpiry(t *testing.T) {
	tc := newTokenCache()
	token := "testToken"
	valid := true

	tc.addToken(token, valid)
	expiryTime := time.Now().Add(-1 * time.Hour)
	tc.setTokenExpiry(token, expiryTime)

	if !tc.isTokenExpired(token, "1s") {
		t.Errorf("Expected token to be expired after setting expiry time")
	}
}

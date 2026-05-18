package auth

import "testing"

func TestHashAndVerify(t *testing.T) {
	hash, err := Hash("correct horse battery staple")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if hash == "" {
		t.Fatal("hash is empty")
	}
	if !Verify(hash, "correct horse battery staple") {
		t.Fatal("Verify should succeed for correct password")
	}
}

func TestVerifyRejectsWrongPassword(t *testing.T) {
	hash, _ := Hash("right")
	if Verify(hash, "wrong") {
		t.Fatal("Verify should fail for wrong password")
	}
}

func TestHashesAreUniquePerCall(t *testing.T) {
	a, _ := Hash("same input")
	b, _ := Hash("same input")
	if a == b {
		t.Fatal("hashes should differ due to random salt")
	}
}

func TestVerifyRejectsMalformedHash(t *testing.T) {
	cases := []string{
		"",
		"not-a-hash",
		"$argon2id$malformed",
		"$bcrypt$2a$10$abc",
	}
	for _, h := range cases {
		if Verify(h, "anything") {
			t.Errorf("Verify should reject malformed hash %q", h)
		}
	}
}

func TestHashRejectsEmptyPassword(t *testing.T) {
	if _, err := Hash(""); err == nil {
		t.Fatal("Hash should reject empty password")
	}
}

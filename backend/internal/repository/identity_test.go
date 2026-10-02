package repository

import "testing"

func TestAuditorPasswordHash(t *testing.T) {
	hash, err := HashAuditorPassword("long-and-unique-passphrase")
	if err != nil {
		t.Fatal(err)
	}
	if !CheckAuditorPassword("long-and-unique-passphrase", hash) {
		t.Fatal("correct password rejected")
	}
	if CheckAuditorPassword("another-password", hash) {
		t.Fatal("wrong password accepted")
	}
	if CheckAuditorPassword("long-and-unique-passphrase", "corrupt") {
		t.Fatal("corrupt hash accepted")
	}
	if _, err := HashAuditorPassword("short"); err == nil {
		t.Fatal("short password accepted")
	}
}

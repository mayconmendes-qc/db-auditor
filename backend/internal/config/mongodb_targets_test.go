package config

import "testing"

func TestValidateMongoTargetURIs(t *testing.T) {
	t.Setenv("AUDITOR_TARGET_ALLOWED_HOSTS", "db.example.com")
	good := "mongodb://reader:secret@db.example.com:27017/app?tls=true&directConnection=true"
	if err := ValidateMongoTargetURIs(map[string]string{"env": good}); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{
		"mongodb://reader:secret@other.example.com:27017/app?tls=true&directConnection=true",
		"mongodb://reader:secret@db.example.com:27017/app?tls=false&directConnection=true",
		"mongodb://reader:secret@db.example.com:27017/app?tls=true&directConnection=false",
		"mongodb://reader:secret@db.example.com:27017/app?tls=true&directConnection=true&tlsInsecure=true",
		"mongodb://reader:secret@db.example.com:27017/?tls=true&directConnection=true",
	} {
		if err := ValidateMongoTargetURIs(map[string]string{"env": raw}); err == nil {
			t.Fatalf("unsafe URI accepted: %q", raw)
		}
	}
}

func TestMongoSlotDoesNotBecomePostgresTarget(t *testing.T) {
	t.Setenv("AUDITOR_TARGET_1_ENGINE", "mongodb")
	t.Setenv("AUDITOR_TARGET_1_ENVIRONMENT_ID", "11111111-1111-4111-8111-111111111111")
	t.Setenv("AUDITOR_TARGET_1_HOST", "db.example.com")
	t.Setenv("AUDITOR_TARGET_1_DATABASE", "app")
	t.Setenv("AUDITOR_TARGET_1_USER", "reader")
	t.Setenv("AUDITOR_TARGET_1_PASSWORD", "test-only-password")
	if _, ok := loadDiscreteTargetDSNs()["11111111-1111-4111-8111-111111111111"]; ok {
		t.Fatal("MongoDB slot was interpreted as PostgreSQL")
	}
	if uri := LoadMongoTargetURIs()["11111111-1111-4111-8111-111111111111"]; uri == "" {
		t.Fatal("MongoDB slot was not loaded")
	}
}

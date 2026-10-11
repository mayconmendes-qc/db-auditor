package config

import "testing"

func TestLoadSeedEnvironmentsDefaults(t *testing.T) {
	rows, err := loadSeedEnvironments(func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].Name != "Demo Tiger Cloud" || rows[1].ID != demoEnvironmentTwo {
		t.Fatalf("defaults = %+v", rows)
	}
}

func TestLoadSeedEnvironmentsFromEnv(t *testing.T) {
	id := "11111111-1111-4111-8111-111111111111"
	env := map[string]string{
		"AUDITOR_TARGET_1_ENVIRONMENT_ID": id,
		"AUDITOR_TARGET_1_NAME":           "Banco principal",
		"AUDITOR_TARGET_1_TYPE":           "self_hosted",
		"AUDITOR_TARGET_1_DISCOVERY":      "single_database",
	}
	rows, err := loadSeedEnvironments(func(key string) string { return env[key] })
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].ID != id || rows[0].Name != "Banco principal" {
		t.Fatalf("rows = %+v", rows)
	}
}

func TestLoadSeedEnvironmentsRejectsBadID(t *testing.T) {
	env := map[string]string{"AUDITOR_TARGET_1_ENVIRONMENT_ID": "nao-e-uuid", "AUDITOR_TARGET_1_NAME": "X"}
	if _, err := loadSeedEnvironments(func(key string) string { return env[key] }); err == nil {
		t.Fatal("expected invalid uuid")
	}
}

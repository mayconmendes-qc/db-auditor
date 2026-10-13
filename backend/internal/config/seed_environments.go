package config

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

const (
	demoEnvironmentOne = "00000000-0000-0000-0000-000000000001"
	demoEnvironmentTwo = "00000000-0000-0000-0000-000000000002"
)

var uuidPattern = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// SeedEnvironment is one audit_environment row created from .env.
type SeedEnvironment struct {
	ID        string
	Name      string
	Type      string
	Discovery string
}

// LoadSeedEnvironments reads AUDITOR_TARGET_N_ENVIRONMENT_ID, _NAME, _TYPE
// and _DISCOVERY. A slot with an id is created. An empty configuration keeps
// the two local demo rows so a fresh install still has somewhere to start.
func LoadSeedEnvironments() ([]SeedEnvironment, error) {
	return loadSeedEnvironments(os.Getenv)
}

func loadSeedEnvironments(getenv func(string) string) ([]SeedEnvironment, error) {
	var rows []SeedEnvironment
	for n := 1; n <= maxTargetSlots; n++ {
		prefix := "AUDITOR_TARGET_" + strconv.Itoa(n) + "_"
		id := strings.TrimSpace(getenv(prefix + "ENVIRONMENT_ID"))
		if id == "" {
			continue
		}
		name := strings.TrimSpace(getenv(prefix + "NAME"))
		if name == "" {
			name = defaultSeedName(n, id)
		}
		kind := strings.TrimSpace(getenv(prefix + "TYPE"))
		if kind == "" {
			kind = defaultSeedType(id)
		}
		discovery := strings.TrimSpace(getenv(prefix + "DISCOVERY"))
		if discovery == "" {
			discovery = defaultSeedDiscovery(id)
		}
		row, err := normalizeSeed(n, id, name, kind, discovery)
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	if len(rows) > 0 {
		return rows, nil
	}
	return []SeedEnvironment{
		{ID: demoEnvironmentOne, Name: "Demo Tiger Cloud", Type: "tiger_cloud", Discovery: "single_database"},
		{ID: demoEnvironmentTwo, Name: "Demo Datacenter", Type: "self_hosted", Discovery: "multi_database"},
	}, nil
}

func normalizeSeed(slot int, id, name, kind, discovery string) (SeedEnvironment, error) {
	if !uuidPattern.MatchString(id) {
		return SeedEnvironment{}, fmt.Errorf("AUDITOR_TARGET_%d_ENVIRONMENT_ID não é um UUID", slot)
	}
	switch kind {
	case "tiger_cloud", "self_hosted":
	default:
		return SeedEnvironment{}, fmt.Errorf("AUDITOR_TARGET_%d_TYPE deve ser tiger_cloud ou self_hosted", slot)
	}
	switch discovery {
	case "single_database", "multi_database":
	default:
		return SeedEnvironment{}, fmt.Errorf("AUDITOR_TARGET_%d_DISCOVERY deve ser single_database ou multi_database", slot)
	}
	return SeedEnvironment{ID: id, Name: name, Type: kind, Discovery: discovery}, nil
}

func defaultSeedName(slot int, id string) string {
	switch id {
	case demoEnvironmentOne:
		return "Demo Tiger Cloud"
	case demoEnvironmentTwo:
		return "Demo Datacenter"
	default:
		return fmt.Sprintf("Ambiente %d", slot)
	}
}

func defaultSeedType(id string) string {
	if id == demoEnvironmentOne {
		return "tiger_cloud"
	}
	return "self_hosted"
}

func defaultSeedDiscovery(id string) string {
	if id == demoEnvironmentTwo {
		return "multi_database"
	}
	return "single_database"
}

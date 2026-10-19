package loganomaly

import (
	"encoding/json"
	"fmt"
	"time"
)

type genericProfile struct {
	timeField    string
	levelField   string
	messageField string
}

func (g genericProfile) Name() string { return ProfileGenerico }

func (g genericProfile) Parse(raw map[string]json.RawMessage) (time.Time, string, string, error) {
	tsStr, ok := jsonString(raw, g.timeField)
	if !ok || tsStr == "" {
		return time.Time{}, "", "", fmt.Errorf("campo de horário %q ausente", g.timeField)
	}
	ts, err := parseFlexibleTime(tsStr)
	if err != nil {
		return time.Time{}, "", "", err
	}
	sev, _ := jsonString(raw, g.levelField)
	level, _ := normalizeLevel(sev)
	msg, ok := jsonString(raw, g.messageField)
	if !ok {
		msg = ""
	}
	return ts, level, msg, nil
}

func (genericProfile) RoleFromJSON(map[string]json.RawMessage) string { return "" }

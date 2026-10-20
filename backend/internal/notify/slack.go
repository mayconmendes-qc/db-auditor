package notify

import (
	"fmt"
	"strings"
)

// SlackPayload is the native Slack incoming-webhook body (text + blocks).
// It must never include SQL definitions, DSNs or passwords.
type SlackPayload struct {
	Text   string       `json:"text"`
	Blocks []SlackBlock `json:"blocks,omitempty"`
}

// SlackBlock is a minimal Block Kit section.
type SlackBlock struct {
	Type string        `json:"type"`
	Text *SlackTextObj `json:"text,omitempty"`
}

// SlackTextObj is mrkdwn/plain_text.
type SlackTextObj struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// DriftPayload is the sanitized drift alert body for generic webhooks.
type DriftPayload struct {
	Event
	ObjectType string `json:"object_type,omitempty"`
	ObjectKey  string `json:"object_key,omitempty"`
	ChangeKind string `json:"change_kind,omitempty"`
	Link       string `json:"link,omitempty"`
}

// FormatSlack builds a Slack payload from a sanitized Event (and optional drift fields).
func FormatSlack(event Event, objectType, objectKey, changeKind, link string) SlackPayload {
	title := strings.TrimSpace(event.Title)
	if title == "" {
		title = event.Kind
	}
	var b strings.Builder
	fmt.Fprintf(&b, "*%s*\n", title)
	fmt.Fprintf(&b, "Ambiente: `%s`\n", event.EnvironmentID)
	if objectKey != "" {
		fmt.Fprintf(&b, "Objeto: `%s` (%s)\n", objectKey, objectType)
	}
	if changeKind != "" {
		fmt.Fprintf(&b, "Mudança: `%s`\n", changeKind)
	}
	if event.Severity != "" {
		fmt.Fprintf(&b, "Severidade: `%s`\n", event.Severity)
	}
	if link != "" {
		fmt.Fprintf(&b, "<%s|Abrir no auditor>", link)
	}
	text := b.String()
	return SlackPayload{
		Text: title,
		Blocks: []SlackBlock{{
			Type: "section",
			Text: &SlackTextObj{Type: "mrkdwn", Text: text},
		}},
	}
}

// BodyForDestination picks generic JSON or Slack format per destination (ADR-0001 D15).
func BodyForDestination(d Destination, event Event, drift *DriftPayload) any {
	if strings.EqualFold(d.Format, "slack") && d.Channel == "webhook" {
		ot, okey, ck, link := "", "", "", ""
		if drift != nil {
			ot, okey, ck, link = drift.ObjectType, drift.ObjectKey, drift.ChangeKind, drift.Link
		}
		return FormatSlack(event, ot, okey, ck, link)
	}
	if drift != nil {
		return *drift
	}
	return event
}

package esl

import "testing"

func TestParseEventDecodesURLEncoding(t *testing.T) {
	raw := map[string]string{
		"Event-Name":                "CHANNEL_ANSWER",
		"Unique-ID":                 "abc-123",
		"Caller-Caller-ID-Number":   "%2B15551234567",
		"Caller-Destination-Number": "%2B15559998888",
	}
	e := ParseEvent(raw)
	if e.Name != "CHANNEL_ANSWER" {
		t.Errorf("name: %q", e.Name)
	}
	if e.UniqueID != "abc-123" {
		t.Errorf("uuid: %q", e.UniqueID)
	}
	if e.CallerNum != "+15551234567" {
		t.Errorf("caller: %q", e.CallerNum)
	}
	if e.CalledNum != "+15559998888" {
		t.Errorf("called: %q", e.CalledNum)
	}
}

func TestParseEventHandlesCanonicalMIMEHeaderKeys(t *testing.T) {
	// eslgo delivers headers via textproto.MIMEHeader which canonicalizes
	// keys: Unique-ID -> Unique-Id, Job-UUID -> Job-Uuid,
	// Caller-Caller-ID-Number -> Caller-Caller-Id-Number.
	raw := map[string]string{
		"Event-Name":                "CHANNEL_ANSWER",
		"Unique-Id":                 "abc-canonical",
		"Caller-Caller-Id-Number":   "%2B15551234567",
		"Caller-Destination-Number": "%2B15559998888",
		"Job-Uuid":                  "job-canonical",
		"Hangup-Cause":              "NORMAL_CLEARING",
	}
	e := ParseEvent(raw)
	if e.UniqueID != "abc-canonical" {
		t.Errorf("uuid: %q", e.UniqueID)
	}
	if e.CallerNum != "+15551234567" {
		t.Errorf("caller: %q", e.CallerNum)
	}
	if e.JobUUID != "job-canonical" {
		t.Errorf("job: %q", e.JobUUID)
	}
	if e.HangupCause != "NORMAL_CLEARING" {
		t.Errorf("cause: %q", e.HangupCause)
	}
}

func TestEventGetReturnsRawHeader(t *testing.T) {
	raw := map[string]string{
		"Event-Name":         "CUSTOM",
		"Event-Subclass":     "avmd%3A%3Abeep",
		"variable_campaign":  "spring",
		"Channel-Call-State": "ACTIVE",
	}
	e := ParseEvent(raw)
	if e.Get("Event-Subclass") != "avmd::beep" {
		t.Errorf("decoded subclass: %q", e.Get("Event-Subclass"))
	}
	if e.Get("variable_campaign") != "spring" {
		t.Errorf("variable: %q", e.Get("variable_campaign"))
	}
}

func TestParseEventHandlesEmpty(t *testing.T) {
	e := ParseEvent(map[string]string{})
	if e.Name != "" || e.Get("anything") != "" {
		t.Errorf("expected empty event, got %+v", e)
	}
}

func TestOriginateVarsFormatsAsCurlyBraces(t *testing.T) {
	v := OriginateVars{
		"origination_caller_id_number": "+18005551212",
		"tenant_id":                    "42",
	}
	s := v.String()
	if s[0] != '{' || s[len(s)-1] != '}' {
		t.Errorf("must be wrapped in braces: %q", s)
	}
	if !contains(s, "origination_caller_id_number='+18005551212'") {
		t.Errorf("missing caller id: %q", s)
	}
	if !contains(s, "tenant_id='42'") {
		t.Errorf("missing tenant_id: %q", s)
	}
}

func TestOriginateVarsEscapesSingleQuotes(t *testing.T) {
	v := OriginateVars{"x": "it's tricky"}
	s := v.String()
	if !contains(s, `x='it\'s tricky'`) {
		t.Errorf("escape failed: %q", s)
	}
}

func TestOriginateVarsEmptyReturnsEmpty(t *testing.T) {
	if (OriginateVars{}).String() != "" {
		t.Error("empty vars must produce empty string")
	}
}

func TestParseHeadersStandard(t *testing.T) {
	in := "Content-Type: command/reply\nReply-Text: +OK accepted\nJob-UUID: abc-123\n"
	out := ParseHeaders(in)
	if out["Content-Type"] != "command/reply" {
		t.Errorf("content-type: %q", out["Content-Type"])
	}
	if out["Reply-Text"] != "+OK accepted" {
		t.Errorf("reply-text: %q", out["Reply-Text"])
	}
	if out["Job-UUID"] != "abc-123" {
		t.Errorf("job-uuid: %q", out["Job-UUID"])
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

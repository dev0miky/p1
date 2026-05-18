package esl

import (
	"net/url"
	"strings"
)

type Event struct {
	Name        string
	UniqueID    string
	CallerNum   string
	CalledNum   string
	HangupCause string
	JobUUID     string
	headers     map[string]string
}

func (e Event) Get(k string) string {
	if e.headers == nil {
		return ""
	}
	return e.headers[k]
}

func ParseEvent(raw map[string]string) Event {
	decoded := make(map[string]string, len(raw))
	for k, v := range raw {
		if dec, err := url.QueryUnescape(v); err == nil {
			decoded[k] = dec
		} else {
			decoded[k] = v
		}
	}
	return Event{
		Name:        decoded["Event-Name"],
		UniqueID:    decoded["Unique-ID"],
		CallerNum:   decoded["Caller-Caller-ID-Number"],
		CalledNum:   decoded["Caller-Destination-Number"],
		HangupCause: decoded["Hangup-Cause"],
		JobUUID:     decoded["Job-UUID"],
		headers:     decoded,
	}
}

func ParseHeaders(s string) map[string]string {
	out := make(map[string]string)
	for _, line := range strings.Split(s, "\n") {
		i := strings.Index(line, ": ")
		if i <= 0 {
			continue
		}
		out[line[:i]] = strings.TrimSpace(line[i+2:])
	}
	return out
}

type OriginateVars map[string]string

func (v OriginateVars) String() string {
	if len(v) == 0 {
		return ""
	}
	parts := make([]string, 0, len(v))
	for k, val := range v {
		parts = append(parts, k+"='"+escapeVar(val)+"'")
	}
	return "{" + strings.Join(parts, ",") + "}"
}

func escapeVar(s string) string {
	return strings.ReplaceAll(s, "'", "\\'")
}

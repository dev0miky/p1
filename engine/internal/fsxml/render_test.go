package fsxml_test

import (
	"encoding/xml"
	"strings"
	"testing"

	"p1/engine/internal/fsxml"
)

func mustRender(t *testing.T, gws []fsxml.GatewayView) string {
	t.Helper()
	out, err := fsxml.RenderSofia(gws)
	if err != nil {
		t.Fatalf("RenderSofia: %v", err)
	}
	return out
}

func mustParseXML(t *testing.T, s string) {
	t.Helper()
	dec := xml.NewDecoder(strings.NewReader(s))
	for {
		_, err := dec.Token()
		if err != nil {
			if err.Error() == "EOF" {
				return
			}
			t.Fatalf("xml parse error: %v\ndoc:\n%s", err, s)
		}
	}
}

func TestRenderSofia_TLSGateway(t *testing.T) {
	gws := []fsxml.GatewayView{
		{
			Name:          "linphone",
			Proxy:         "sip.linphone.org",
			Username:      "user1",
			Password:      "secret",
			Realm:         "sip.linphone.org",
			Register:      true,
			Transport:     "tls",
			ExpireSeconds: 3600,
			RetrySeconds:  30,
		},
	}
	out := mustRender(t, gws)

	if !strings.Contains(out, `<gateway name="linphone">`) {
		t.Error("missing gateway name")
	}
	if !strings.Contains(out, `<param name="username" value="user1"`) {
		t.Error("missing username param")
	}
	if !strings.Contains(out, `<param name="password" value="secret"`) {
		t.Error("missing password param")
	}
	if !strings.Contains(out, `;transport=tls`) {
		t.Error("missing ;transport=tls in proxy")
	}
	if !strings.Contains(out, `<param name="register" value="true"`) {
		t.Error("missing register=true")
	}
	if !strings.Contains(out, `<param name="register-transport" value="tls"`) {
		t.Error("missing register-transport for tls")
	}
	mustParseXML(t, out)
}

func TestRenderSofia_NoCredentials(t *testing.T) {
	gws := []fsxml.GatewayView{
		{
			Name:          "carrier",
			Proxy:         "sip.carrier.com",
			Register:      false,
			ExpireSeconds: 1800,
			RetrySeconds:  60,
		},
	}
	out := mustRender(t, gws)

	if strings.Contains(out, `<param name="username"`) {
		t.Error("should not emit username when empty")
	}
	if strings.Contains(out, `<param name="password"`) {
		t.Error("should not emit password when empty")
	}
	if !strings.Contains(out, `<param name="register" value="false"`) {
		t.Error("missing register=false")
	}
	mustParseXML(t, out)
}

func TestRenderSofia_Escaping(t *testing.T) {
	gws := []fsxml.GatewayView{
		{
			Name:          "badchars",
			Proxy:         "sip.example.com",
			Username:      `user&1`,
			Password:      `a&b"<c>`,
			Realm:         `realm<x>`,
			Register:      true,
			ExpireSeconds: 300,
			RetrySeconds:  30,
		},
	}
	out := mustRender(t, gws)

	if strings.Contains(out, `a&b"<c>`) {
		t.Error("raw special chars in password")
	}
	if strings.Contains(out, `user&1`) {
		t.Error("raw & in username")
	}
	mustParseXML(t, out)
}

func TestRenderSofia_ZeroGateways(t *testing.T) {
	out := mustRender(t, nil)

	mustParseXML(t, out)

	if !strings.Contains(out, `<profile name="external">`) {
		t.Error("missing external profile")
	}
	if !strings.Contains(out, `sip-port`) {
		t.Error("external settings not present")
	}
}

func TestNotFound(t *testing.T) {
	out := fsxml.NotFound()
	if !strings.Contains(out, `name="result"`) {
		t.Error("missing result section")
	}
	mustParseXML(t, out)
}

func TestRenderSofia_FSPreprocessorTokens(t *testing.T) {
	out := mustRender(t, nil)

	if strings.Contains(out, `${${`) {
		t.Error("output contains broken ${${ token — FS preprocessor vars must use $${...} form")
	}
	if !strings.Contains(out, `$${local_ip_v4}`) {
		t.Error("output missing $${local_ip_v4}")
	}
	if !strings.Contains(out, `$${external_sip_port}`) {
		t.Error("output missing $${external_sip_port}")
	}

	mustParseXML(t, out)
}

package fsxml_test

import (
	"encoding/xml"
	"strings"
	"testing"

	"p1/engine/internal/fsxml"
)

func mustRenderGW(t *testing.T, g fsxml.GatewayView) string {
	t.Helper()
	out, err := fsxml.RenderGatewayFile(g)
	if err != nil {
		t.Fatalf("RenderGatewayFile: %v", err)
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

func TestRenderGatewayFile_UDPRegisterSrtp(t *testing.T) {
	out := mustRenderGW(t, fsxml.GatewayView{
		Name:            "linphone",
		Proxy:           "sip.linphone.org",
		Username:        "user1",
		Password:        "secret",
		Register:        true,
		Transport:       "udp",
		MediaEncryption: "srtp",
		CallerIDInFrom:  true,
		ExpireSeconds:   3600,
		RetrySeconds:    30,
	})

	if !strings.Contains(out, `<include>`) {
		t.Error("missing <include>")
	}
	if !strings.Contains(out, `<gateway name="linphone">`) {
		t.Error("missing gateway name")
	}
	if !strings.Contains(out, `<param name="register-transport" value="udp"/>`) {
		t.Errorf("missing register-transport=udp\ndoc:\n%s", out)
	}
	if !strings.Contains(out, `<variable name="rtp_secure_media" value="mandatory:AES_CM_128_HMAC_SHA1_80"/>`) {
		t.Errorf("missing rtp_secure_media variable\ndoc:\n%s", out)
	}
	if strings.Contains(out, ";transport=tls") {
		t.Error("udp gateway must not have ;transport=tls in proxy")
	}
	mustParseXML(t, out)
}

func TestRenderGatewayFile_TLS(t *testing.T) {
	out := mustRenderGW(t, fsxml.GatewayView{
		Name:          "linphone-tls",
		Proxy:         "sip.linphone.org",
		Username:      "user1",
		Password:      "secret",
		Realm:         "sip.linphone.org",
		Register:      true,
		Transport:     "tls",
		ExpireSeconds: 3600,
		RetrySeconds:  30,
	})

	if !strings.Contains(out, `;transport=tls`) {
		t.Error("tls gateway must have ;transport=tls in proxy")
	}
	if !strings.Contains(out, `<param name="register-transport" value="tls"/>`) {
		t.Errorf("tls gateway must have register-transport=tls\ndoc:\n%s", out)
	}
	mustParseXML(t, out)
}

func TestRenderGatewayFile_Escaping(t *testing.T) {
	out := mustRenderGW(t, fsxml.GatewayView{
		Name:          "escape-gw",
		Proxy:         "sip.example.com",
		Username:      `user&1`,
		Password:      `a&b"<c>`,
		Realm:         `realm<x>`,
		Register:      true,
		Transport:     "udp",
		ExpireSeconds: 300,
		RetrySeconds:  30,
	})

	if strings.Contains(out, `a&b"<c>`) {
		t.Error("raw special chars in password")
	}
	if strings.Contains(out, `user&1`) {
		t.Error("raw & in username")
	}
	mustParseXML(t, out)
}

func TestRenderGatewayFile_NoneEncryption(t *testing.T) {
	out := mustRenderGW(t, fsxml.GatewayView{
		Name:            "plain-gw",
		Proxy:           "sip.example.com",
		Register:        true,
		Transport:       "udp",
		MediaEncryption: "none",
		ExpireSeconds:   3600,
		RetrySeconds:    30,
	})

	if strings.Contains(out, "rtp_secure_media") {
		t.Errorf("none encryption must not emit rtp_secure_media\ndoc:\n%s", out)
	}
	mustParseXML(t, out)
}

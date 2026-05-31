package gateway

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"p1/engine/internal/fsxml"
)

func TestReloadCommands(t *testing.T) {
	cmds := ReloadCommands("linphone")
	want := [2]string{
		"sofia profile external killgw linphone",
		"sofia profile external rescan",
	}
	if cmds != want {
		t.Fatalf("got %v, want %v", cmds, want)
	}
}

func TestParseRegisterStatus(t *testing.T) {
	allowed := map[string]bool{
		"unknown":    true,
		"registered": true,
		"trying":     true,
		"failed":     true,
		"noreg":      true,
		"down":       true,
	}

	cases := []struct {
		input string
		want  string
	}{
		{"Name\tlinphone\nState\tREGED\nStatus\tUP\n", "registered"},
		{"State\tTRYING\n", "trying"},
		{"State\tFAIL_WAIT\n", "failed"},
		{"State\tREGFAIL\n", "failed"},
		{"State\tNOREG\n", "noreg"},
		{"State\tDOWN\n", "down"},
		{"Invalid gateway!\n", "failed"},
		{"garbage no state line\n", "unknown"},
	}

	for _, tc := range cases {
		got := ParseRegisterStatus(tc.input)
		if got != tc.want {
			t.Errorf("ParseRegisterStatus(%q) = %q, want %q", tc.input, got, tc.want)
		}
		if !allowed[got] {
			t.Errorf("ParseRegisterStatus(%q) returned %q which is not in DB-allowed set", tc.input, got)
		}
	}
}

func TestParseRegisterStatusAllowedSet(t *testing.T) {
	allowed := map[string]bool{
		"unknown":    true,
		"registered": true,
		"trying":     true,
		"failed":     true,
		"noreg":      true,
		"down":       true,
	}

	probes := []string{
		"Name\tlinphone\nState\tREGED\n",
		"State\tTRYING\n",
		"State\tREGISTER\n",
		"State\tFAIL_WAIT\n",
		"State\tREGFAIL\n",
		"State\tFAILED\n",
		"State\tEXPIRED\n",
		"State\tNOREG\n",
		"State\tUNREGED\n",
		"State\tDOWN\n",
		"State\tNOAVAIL\n",
		"State\tWHATEVER\n",
		"Invalid gateway!\n",
		"",
	}
	for _, input := range probes {
		got := ParseRegisterStatus(input)
		if !allowed[got] {
			t.Errorf("ParseRegisterStatus(%q) = %q outside DB-allowed set", input, got)
		}
	}
}

func newTestProvisioner(t *testing.T) (*Provisioner, string) {
	t.Helper()
	dir := t.TempDir()
	p := &Provisioner{gatewayDir: dir}
	return p, dir
}

func TestProvisionerWriteFile(t *testing.T) {
	p, dir := newTestProvisioner(t)

	view := fsxml.GatewayView{
		Name:            "linphone",
		Proxy:           "sip.linphone.org",
		Username:        "user1",
		Password:        "s3cret",
		Realm:           "sip.linphone.org",
		Register:        true,
		Transport:       "udp",
		MediaEncryption: "srtp",
		CallerIDInFrom:  true,
		ExpireSeconds:   3600,
		RetrySeconds:    30,
	}
	if err := p.writeFile(view); err != nil {
		t.Fatalf("writeFile: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "linphone.xml"))
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, `<include>`) {
		t.Error("missing <include>")
	}
	if !strings.Contains(content, `<gateway name="linphone">`) {
		t.Error("missing gateway name")
	}
	if !strings.Contains(content, `<param name="register-transport" value="udp"/>`) {
		t.Errorf("missing register-transport=udp\ndoc:\n%s", content)
	}
	if !strings.Contains(content, `<variable name="rtp_secure_media" value="mandatory:AES_CM_128_HMAC_SHA1_80"/>`) {
		t.Errorf("missing rtp_secure_media\ndoc:\n%s", content)
	}
}

func TestProvisionerWriteFileOverwrites(t *testing.T) {
	p, dir := newTestProvisioner(t)

	v1 := fsxml.GatewayView{
		Name:          "gw",
		Proxy:         "sip.old.com",
		Transport:     "udp",
		ExpireSeconds: 3600,
		RetrySeconds:  30,
	}
	if err := p.writeFile(v1); err != nil {
		t.Fatalf("writeFile first: %v", err)
	}

	v2 := fsxml.GatewayView{
		Name:          "gw",
		Proxy:         "sip.new.com",
		Transport:     "udp",
		ExpireSeconds: 3600,
		RetrySeconds:  30,
	}
	if err := p.writeFile(v2); err != nil {
		t.Fatalf("writeFile second: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "gw.xml"))
	if !strings.Contains(string(data), "sip.new.com") {
		t.Errorf("file not overwritten: %s", string(data))
	}
}

func TestProvisionerWriteFileMode(t *testing.T) {
	p, dir := newTestProvisioner(t)

	view := fsxml.GatewayView{
		Name:          "mode-gw",
		Proxy:         "sip.example.com",
		Transport:     "udp",
		ExpireSeconds: 3600,
		RetrySeconds:  30,
	}
	if err := p.writeFile(view); err != nil {
		t.Fatalf("writeFile: %v", err)
	}
	info, err := os.Stat(filepath.Join(dir, "mode-gw.xml"))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("file mode: got %04o, want 0600", got)
	}
}

func TestProvisionerRemoveFile(t *testing.T) {
	p, dir := newTestProvisioner(t)

	view := fsxml.GatewayView{
		Name:          "del-gw",
		Proxy:         "sip.example.com",
		Transport:     "udp",
		ExpireSeconds: 3600,
		RetrySeconds:  30,
	}
	if err := p.writeFile(view); err != nil {
		t.Fatalf("writeFile: %v", err)
	}
	path := filepath.Join(dir, "del-gw.xml")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file should exist after writeFile: %v", err)
	}

	if err := p.removeFile("del-gw"); err != nil {
		t.Fatalf("removeFile: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("file should be gone after removeFile")
	}
}

func TestProvisionerRemoveNotExistIsNoop(t *testing.T) {
	p, _ := newTestProvisioner(t)

	if err := p.removeFile("nonexistent"); err != nil {
		t.Fatalf("removeFile on missing file should be noop: %v", err)
	}
}

package gateway

import (
	"testing"
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

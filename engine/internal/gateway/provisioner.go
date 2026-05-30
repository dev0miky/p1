package gateway

import (
	"context"
	"strings"

	"p1/engine/internal/esl"
)

func ReloadCommands(name string) [2]string {
	return [2]string{
		"sofia profile external killgw " + name,
		"sofia profile external rescan",
	}
}

func ParseRegisterStatus(eslOutput string) string {
	if strings.Contains(eslOutput, "Invalid gateway") {
		return "failed"
	}
	for _, line := range strings.Split(eslOutput, "\n") {
		if strings.HasPrefix(line, "State") {
			f := strings.Fields(line)
			if len(f) >= 2 {
				return normalizeSofiaState(f[1])
			}
		}
	}
	return "unknown"
}

func normalizeSofiaState(s string) string {
	switch strings.ToUpper(s) {
	case "REGED":
		return "registered"
	case "TRYING", "REGISTER":
		return "trying"
	case "FAIL_WAIT", "REGFAIL", "FAILED", "EXPIRED":
		return "failed"
	case "NOREG", "UNREGED":
		return "noreg"
	case "DOWN", "NOAVAIL":
		return "down"
	default:
		return "unknown"
	}
}

type Provisioner struct{ esl *esl.Client }

func NewProvisioner(c *esl.Client) *Provisioner { return &Provisioner{esl: c} }

func (p *Provisioner) Reload(ctx context.Context, name string) error {
	for _, cmd := range ReloadCommands(name) {
		if _, err := p.esl.API(ctx, cmd); err != nil {
			return err
		}
	}
	return nil
}

func (p *Provisioner) Status(ctx context.Context, name string) (string, error) {
	out, err := p.esl.API(ctx, "sofia status gateway "+name)
	if err != nil {
		return "", err
	}
	return ParseRegisterStatus(out), nil
}

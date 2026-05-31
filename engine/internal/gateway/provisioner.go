package gateway

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jackc/pgx/v5"

	"p1/engine/internal/db"
	"p1/engine/internal/esl"
	"p1/engine/internal/fsxml"
)

// full profile restart is the only reliable reload for changed gateway include files;
// killgw+rescan leaves stale state when a file was modified in place.
func ReloadCommands(_ string) []string {
	return []string{"sofia profile external restart reloadxml"}
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

func ToView(g Gateway) fsxml.GatewayView {
	v := fsxml.GatewayView{
		Proxy:           g.Proxy,
		Name:            g.Name,
		Register:        g.Register,
		Transport:       string(g.Transport),
		MediaEncryption: g.MediaEncryption,
		CallerIDInFrom:  g.CallerIDInFrom,
		ExpireSeconds:   g.ExpireSeconds,
		RetrySeconds:    g.RetrySeconds,
		Extra:           g.ExtraParams,
	}
	if g.Username != nil {
		v.Username = *g.Username
	}
	if g.Password != nil {
		v.Password = *g.Password
	}
	if g.Realm != nil {
		v.Realm = *g.Realm
	}
	if g.FromUser != nil {
		v.FromUser = *g.FromUser
	}
	if g.FromDomain != nil {
		v.FromDomain = *g.FromDomain
	}
	return v
}

type Provisioner struct {
	esl        *esl.Client
	gatewayDir string
}

func NewProvisioner(c *esl.Client, gatewayDir string) *Provisioner {
	return &Provisioner{esl: c, gatewayDir: gatewayDir}
}

func (p *Provisioner) writeFile(view fsxml.GatewayView) error {
	xml, err := fsxml.RenderGatewayFile(view)
	if err != nil {
		return fmt.Errorf("render gateway file: %w", err)
	}
	if err := os.MkdirAll(p.gatewayDir, 0700); err != nil {
		return fmt.Errorf("ensure gateway dir: %w", err)
	}
	dest := filepath.Join(p.gatewayDir, view.Name+".xml")
	tmp := dest + ".tmp"
	if err := os.WriteFile(tmp, []byte(xml), 0600); err != nil {
		return fmt.Errorf("write gateway tmp file: %w", err)
	}
	if err := os.Rename(tmp, dest); err != nil {
		return fmt.Errorf("rename gateway file: %w", err)
	}
	return nil
}

func (p *Provisioner) Reload(ctx context.Context, name string) error {
	for _, cmd := range ReloadCommands(name) {
		if _, err := p.esl.API(ctx, cmd); err != nil {
			return err
		}
	}
	return nil
}

func (p *Provisioner) Sync(ctx context.Context, view fsxml.GatewayView) error {
	if err := p.writeFile(view); err != nil {
		return err
	}
	return p.Reload(ctx, view.Name)
}

func (p *Provisioner) removeFile(name string) error {
	path := filepath.Join(p.gatewayDir, name+".xml")
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove gateway file: %w", err)
	}
	return nil
}

func (p *Provisioner) Remove(ctx context.Context, name string) error {
	if err := p.removeFile(name); err != nil {
		return err
	}
	return p.Reload(ctx, name)
}

func (p *Provisioner) Status(ctx context.Context, name string) (string, error) {
	out, err := p.esl.API(ctx, "sofia status gateway "+name)
	if err != nil {
		return "", err
	}
	return ParseRegisterStatus(out), nil
}

func SyncAll(ctx context.Context, pool *db.Pool, repo *Repo, prov *Provisioner) error {
	var gws []Gateway
	if err := db.WithCtx(ctx, pool, db.Ctx{Role: "super_admin"}, func(tx pgx.Tx) error {
		var err error
		gws, err = repo.ListEnabledWithSecretTx(ctx, tx)
		return err
	}); err != nil {
		return fmt.Errorf("list enabled gateways: %w", err)
	}
	for _, g := range gws {
		if err := prov.writeFile(ToView(g)); err != nil {
			return fmt.Errorf("sync gateway %q: %w", g.Name, err)
		}
	}
	// single rescan after all files are written
	if len(gws) > 0 {
		if _, err := prov.esl.API(ctx, "sofia profile external rescan"); err != nil {
			return fmt.Errorf("rescan: %w", err)
		}
	}
	return nil
}

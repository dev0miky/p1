package gateway_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"

	"p1/engine/internal/db"
	"p1/engine/internal/gateway"
	"p1/engine/internal/testutil"
)

const testEncKey = "test-enc-key-0123456789"

func superCtx() db.Ctx { return db.Ctx{Role: "super_admin"} }

func ptr[T any](v T) *T { return &v }

func makeGateway(name string) gateway.Gateway {
	return gateway.Gateway{
		Name:           name,
		Proxy:          "sip.example.com",
		Register:       true,
		Username:       ptr("user1"),
		Transport:      gateway.TransportUDP,
		ExpireSeconds:  3600,
		RetrySeconds:   30,
		CallerIDInFrom: true,
		Enabled:        true,
	}
}

func TestCreateGetRoundTrip(t *testing.T) {
	pool := testutil.TestPool(t)
	repo := gateway.NewRepo(testEncKey)
	ctx := context.Background()

	var created gateway.Gateway
	if err := db.WithCtx(ctx, pool, superCtx(), func(tx pgx.Tx) error {
		g, err := repo.CreateTx(ctx, tx, makeGateway("rtrip-a"))
		created = g
		return err
	}); err != nil {
		t.Fatalf("create: %v", err)
	}

	if created.Name != "rtrip-a" {
		t.Fatalf("name: got %q", created.Name)
	}
	if created.HasPassword {
		t.Fatal("expected HasPassword false when no password given")
	}
	if created.RegisterStatus != "unknown" {
		t.Fatalf("default register_status: got %q", created.RegisterStatus)
	}

	var fetched gateway.Gateway
	if err := db.WithCtx(ctx, pool, superCtx(), func(tx pgx.Tx) error {
		g, err := repo.GetTx(ctx, tx, created.ID)
		fetched = g
		return err
	}); err != nil {
		t.Fatalf("get: %v", err)
	}

	if fetched.ID != created.ID || fetched.Proxy != "sip.example.com" {
		t.Fatalf("round-trip mismatch: %+v", fetched)
	}
	if fetched.Password != nil {
		t.Fatal("GetTx must not populate Password")
	}
}

func TestHasPasswordTrueWhenPasswordGiven(t *testing.T) {
	pool := testutil.TestPool(t)
	repo := gateway.NewRepo(testEncKey)
	ctx := context.Background()

	g := makeGateway("haspw-a")
	g.Password = ptr("s3cret")

	var created gateway.Gateway
	if err := db.WithCtx(ctx, pool, superCtx(), func(tx pgx.Tx) error {
		c, err := repo.CreateTx(ctx, tx, g)
		created = c
		return err
	}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if !created.HasPassword {
		t.Fatal("expected HasPassword true")
	}
}

func TestPasswordEncryptDecrypt(t *testing.T) {
	pool := testutil.TestPool(t)
	repo := gateway.NewRepo(testEncKey)
	ctx := context.Background()

	g := makeGateway("crypta")
	pw := "sup3rSecr3t"
	g.Password = &pw

	var id int64
	if err := db.WithCtx(ctx, pool, superCtx(), func(tx pgx.Tx) error {
		c, err := repo.CreateTx(ctx, tx, g)
		id = c.ID
		return err
	}); err != nil {
		t.Fatalf("create: %v", err)
	}

	// GetTx never populates Password
	if err := db.WithCtx(ctx, pool, superCtx(), func(tx pgx.Tx) error {
		got, err := repo.GetTx(ctx, tx, id)
		if err != nil {
			return err
		}
		if got.Password != nil {
			t.Fatalf("GetTx leaked password: %v", *got.Password)
		}
		return nil
	}); err != nil {
		t.Fatalf("get: %v", err)
	}

	// GetWithSecretTx decrypts to plaintext
	if err := db.WithCtx(ctx, pool, superCtx(), func(tx pgx.Tx) error {
		got, err := repo.GetWithSecretTx(ctx, tx, id)
		if err != nil {
			return err
		}
		if got.Password == nil || *got.Password != pw {
			t.Fatalf("password did not round-trip: %v", got.Password)
		}
		return nil
	}); err != nil {
		t.Fatalf("get-with-secret: %v", err)
	}

	// ListTx never populates Password
	if err := db.WithCtx(ctx, pool, superCtx(), func(tx pgx.Tx) error {
		list, err := repo.ListTx(ctx, tx)
		if err != nil {
			return err
		}
		for _, gw := range list {
			if gw.Password != nil {
				t.Fatalf("ListTx leaked password for %q", gw.Name)
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("list: %v", err)
	}
}

func TestUpdateChangesFields(t *testing.T) {
	pool := testutil.TestPool(t)
	repo := gateway.NewRepo(testEncKey)
	ctx := context.Background()

	var id int64
	if err := db.WithCtx(ctx, pool, superCtx(), func(tx pgx.Tx) error {
		c, err := repo.CreateTx(ctx, tx, makeGateway("upd-a"))
		id = c.ID
		return err
	}); err != nil {
		t.Fatalf("create: %v", err)
	}

	var updated gateway.Gateway
	if err := db.WithCtx(ctx, pool, superCtx(), func(tx pgx.Tx) error {
		g, err := repo.UpdateTx(ctx, tx, id, gateway.UpdatePatch{
			Proxy: "sip.new.com",
		})
		updated = g
		return err
	}); err != nil {
		t.Fatalf("update: %v", err)
	}

	if updated.Proxy != "sip.new.com" {
		t.Fatalf("proxy not updated: got %q", updated.Proxy)
	}
}

func TestUpdateNilPasswordLeavesExisting(t *testing.T) {
	pool := testutil.TestPool(t)
	repo := gateway.NewRepo(testEncKey)
	ctx := context.Background()

	g := makeGateway("upd-pw")
	orig := "original-pw"
	g.Password = &orig

	var id int64
	if err := db.WithCtx(ctx, pool, superCtx(), func(tx pgx.Tx) error {
		c, err := repo.CreateTx(ctx, tx, g)
		id = c.ID
		return err
	}); err != nil {
		t.Fatalf("create: %v", err)
	}

	// update with nil password — must not clear existing password
	if err := db.WithCtx(ctx, pool, superCtx(), func(tx pgx.Tx) error {
		_, err := repo.UpdateTx(ctx, tx, id, gateway.UpdatePatch{Proxy: "sip.example.com"})
		return err
	}); err != nil {
		t.Fatalf("update: %v", err)
	}

	// original password still decrypts
	if err := db.WithCtx(ctx, pool, superCtx(), func(tx pgx.Tx) error {
		got, err := repo.GetWithSecretTx(ctx, tx, id)
		if err != nil {
			return err
		}
		if got.Password == nil || *got.Password != orig {
			t.Fatalf("password was cleared or changed: %v", got.Password)
		}
		return nil
	}); err != nil {
		t.Fatalf("get-with-secret: %v", err)
	}
}

func TestUpdatePasswordReplaces(t *testing.T) {
	pool := testutil.TestPool(t)
	repo := gateway.NewRepo(testEncKey)
	ctx := context.Background()

	g := makeGateway("upd-pw2")
	orig := "old-password"
	g.Password = &orig

	var id int64
	if err := db.WithCtx(ctx, pool, superCtx(), func(tx pgx.Tx) error {
		c, err := repo.CreateTx(ctx, tx, g)
		id = c.ID
		return err
	}); err != nil {
		t.Fatalf("create: %v", err)
	}

	newPw := "new-password"
	if err := db.WithCtx(ctx, pool, superCtx(), func(tx pgx.Tx) error {
		_, err := repo.UpdateTx(ctx, tx, id, gateway.UpdatePatch{Password: &newPw})
		return err
	}); err != nil {
		t.Fatalf("update: %v", err)
	}

	if err := db.WithCtx(ctx, pool, superCtx(), func(tx pgx.Tx) error {
		got, err := repo.GetWithSecretTx(ctx, tx, id)
		if err != nil {
			return err
		}
		if got.Password == nil || *got.Password != newPw {
			t.Fatalf("password not replaced: %v", got.Password)
		}
		return nil
	}); err != nil {
		t.Fatalf("get-with-secret: %v", err)
	}
}

func TestActivateTxMakesOneActive(t *testing.T) {
	pool := testutil.TestPool(t)
	repo := gateway.NewRepo(testEncKey)
	ctx := context.Background()

	var idA, idB int64
	if err := db.WithCtx(ctx, pool, superCtx(), func(tx pgx.Tx) error {
		a, err := repo.CreateTx(ctx, tx, makeGateway("act-a"))
		if err != nil {
			return err
		}
		idA = a.ID
		b, err := repo.CreateTx(ctx, tx, makeGateway("act-b"))
		if err != nil {
			return err
		}
		idB = b.ID
		return nil
	}); err != nil {
		t.Fatalf("create: %v", err)
	}

	// activate A first
	if err := db.WithCtx(ctx, pool, superCtx(), func(tx pgx.Tx) error {
		return repo.ActivateTx(ctx, tx, idA)
	}); err != nil {
		t.Fatalf("activate A: %v", err)
	}

	// activate B — only B should be active
	if err := db.WithCtx(ctx, pool, superCtx(), func(tx pgx.Tx) error {
		return repo.ActivateTx(ctx, tx, idB)
	}); err != nil {
		t.Fatalf("activate B: %v", err)
	}

	if err := db.WithCtx(ctx, pool, superCtx(), func(tx pgx.Tx) error {
		a, err := repo.GetTx(ctx, tx, idA)
		if err != nil {
			return err
		}
		b, err := repo.GetTx(ctx, tx, idB)
		if err != nil {
			return err
		}
		if a.IsActive {
			t.Fatal("A should not be active after activating B")
		}
		if !b.IsActive {
			t.Fatal("B should be active")
		}
		return nil
	}); err != nil {
		t.Fatalf("get: %v", err)
	}
}

func TestSetStatusTx(t *testing.T) {
	pool := testutil.TestPool(t)
	repo := gateway.NewRepo(testEncKey)
	ctx := context.Background()

	var id int64
	if err := db.WithCtx(ctx, pool, superCtx(), func(tx pgx.Tx) error {
		c, err := repo.CreateTx(ctx, tx, makeGateway("status-a"))
		id = c.ID
		return err
	}); err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := db.WithCtx(ctx, pool, superCtx(), func(tx pgx.Tx) error {
		return repo.SetStatusTx(ctx, tx, "status-a", "registered")
	}); err != nil {
		t.Fatalf("set status: %v", err)
	}

	if err := db.WithCtx(ctx, pool, superCtx(), func(tx pgx.Tx) error {
		g, err := repo.GetTx(ctx, tx, id)
		if err != nil {
			return err
		}
		if g.RegisterStatus != "registered" {
			t.Fatalf("status: got %q", g.RegisterStatus)
		}
		if g.RegisterStatusAt == nil {
			t.Fatal("register_status_at should be set")
		}
		return nil
	}); err != nil {
		t.Fatalf("get: %v", err)
	}
}

func TestSetStatusTxNotFound(t *testing.T) {
	pool := testutil.TestPool(t)
	repo := gateway.NewRepo(testEncKey)
	ctx := context.Background()

	if err := db.WithCtx(ctx, pool, superCtx(), func(tx pgx.Tx) error {
		return repo.SetStatusTx(ctx, tx, "no-such-gateway", "registered")
	}); err != gateway.ErrNotFound {
		t.Fatalf("expected ErrNotFound for non-existent name, got: %v", err)
	}
}

func TestActiveNameTx(t *testing.T) {
	pool := testutil.TestPool(t)
	repo := gateway.NewRepo(testEncKey)
	ctx := context.Background()

	// none active yet
	if err := db.WithCtx(ctx, pool, superCtx(), func(tx pgx.Tx) error {
		name, err := repo.ActiveNameTx(ctx, tx)
		if err != nil {
			return err
		}
		if name != "" {
			t.Fatalf("expected empty, got %q", name)
		}
		return nil
	}); err != nil {
		t.Fatalf("active name: %v", err)
	}

	var id int64
	if err := db.WithCtx(ctx, pool, superCtx(), func(tx pgx.Tx) error {
		c, err := repo.CreateTx(ctx, tx, makeGateway("active-name"))
		id = c.ID
		return err
	}); err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := db.WithCtx(ctx, pool, superCtx(), func(tx pgx.Tx) error {
		return repo.ActivateTx(ctx, tx, id)
	}); err != nil {
		t.Fatalf("activate: %v", err)
	}

	if err := db.WithCtx(ctx, pool, superCtx(), func(tx pgx.Tx) error {
		name, err := repo.ActiveNameTx(ctx, tx)
		if err != nil {
			return err
		}
		if name != "active-name" {
			t.Fatalf("expected active-name, got %q", name)
		}
		return nil
	}); err != nil {
		t.Fatalf("active name after activate: %v", err)
	}
}

func TestCiphertextAtRest(t *testing.T) {
	pool := testutil.TestPool(t)
	repo := gateway.NewRepo(testEncKey)
	ctx := context.Background()

	plaintext := "at-rest-secret"
	g := makeGateway("cipher-a")
	g.Password = &plaintext

	var id int64
	if err := db.WithCtx(ctx, pool, superCtx(), func(tx pgx.Tx) error {
		c, err := repo.CreateTx(ctx, tx, g)
		id = c.ID
		return err
	}); err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := db.WithCtx(ctx, pool, superCtx(), func(tx pgx.Tx) error {
		var raw []byte
		if err := tx.QueryRow(ctx, `SELECT password_enc FROM gateways WHERE id=$1`, id).Scan(&raw); err != nil {
			return err
		}
		if len(raw) == 0 {
			t.Fatal("password_enc is empty — not stored")
		}
		if string(raw) == plaintext {
			t.Fatal("password_enc equals plaintext — not encrypted")
		}
		return nil
	}); err != nil {
		t.Fatalf("raw read: %v", err)
	}
}

func TestListEnabledWithSecretTx(t *testing.T) {
	pool := testutil.TestPool(t)
	repo := gateway.NewRepo(testEncKey)
	ctx := context.Background()

	pw := "enabled-secret"
	enabled := makeGateway("lews-enabled")
	enabled.Password = &pw

	disabled := makeGateway("lews-disabled")
	disabled.Enabled = false

	var enabledID int64
	if err := db.WithCtx(ctx, pool, superCtx(), func(tx pgx.Tx) error {
		e, err := repo.CreateTx(ctx, tx, enabled)
		if err != nil {
			return err
		}
		enabledID = e.ID
		_, err = repo.CreateTx(ctx, tx, disabled)
		return err
	}); err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := db.WithCtx(ctx, pool, superCtx(), func(tx pgx.Tx) error {
		list, err := repo.ListEnabledWithSecretTx(ctx, tx)
		if err != nil {
			return err
		}
		if len(list) != 1 {
			t.Fatalf("expected 1 enabled gateway, got %d", len(list))
		}
		if list[0].ID != enabledID {
			t.Fatalf("wrong gateway returned: got id %d, want %d", list[0].ID, enabledID)
		}
		if list[0].Password == nil || *list[0].Password != pw {
			t.Fatalf("password not decrypted: %v", list[0].Password)
		}
		return nil
	}); err != nil {
		t.Fatalf("list enabled with secret: %v", err)
	}
}

func TestDeleteTx(t *testing.T) {
	pool := testutil.TestPool(t)
	repo := gateway.NewRepo(testEncKey)
	ctx := context.Background()

	var id int64
	if err := db.WithCtx(ctx, pool, superCtx(), func(tx pgx.Tx) error {
		c, err := repo.CreateTx(ctx, tx, makeGateway("del-a"))
		id = c.ID
		return err
	}); err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := db.WithCtx(ctx, pool, superCtx(), func(tx pgx.Tx) error {
		return repo.DeleteTx(ctx, tx, id)
	}); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if err := db.WithCtx(ctx, pool, superCtx(), func(tx pgx.Tx) error {
		_, err := repo.GetTx(ctx, tx, id)
		return err
	}); err != gateway.ErrNotFound {
		t.Fatalf("expected ErrNotFound after delete, got: %v", err)
	}

	if err := db.WithCtx(ctx, pool, superCtx(), func(tx pgx.Tx) error {
		return repo.DeleteTx(ctx, tx, id)
	}); err != gateway.ErrNotFound {
		t.Fatalf("expected ErrNotFound deleting non-existent, got: %v", err)
	}
}

func TestErrNotFoundMapping(t *testing.T) {
	pool := testutil.TestPool(t)
	repo := gateway.NewRepo(testEncKey)
	ctx := context.Background()

	const bogusID = int64(999999999)

	if err := db.WithCtx(ctx, pool, superCtx(), func(tx pgx.Tx) error {
		_, err := repo.GetTx(ctx, tx, bogusID)
		return err
	}); err != gateway.ErrNotFound {
		t.Fatalf("GetTx non-existent: expected ErrNotFound, got: %v", err)
	}

	if err := db.WithCtx(ctx, pool, superCtx(), func(tx pgx.Tx) error {
		_, err := repo.UpdateTx(ctx, tx, bogusID, gateway.UpdatePatch{Proxy: "sip.x.com"})
		return err
	}); err != gateway.ErrNotFound {
		t.Fatalf("UpdateTx non-existent: expected ErrNotFound, got: %v", err)
	}
}

func TestMediaEncryptionRoundTrip(t *testing.T) {
	pool := testutil.TestPool(t)
	repo := gateway.NewRepo(testEncKey)
	ctx := context.Background()

	g := makeGateway("media-enc-srtp")
	g.MediaEncryption = "srtp"

	var created gateway.Gateway
	if err := db.WithCtx(ctx, pool, superCtx(), func(tx pgx.Tx) error {
		c, err := repo.CreateTx(ctx, tx, g)
		created = c
		return err
	}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.MediaEncryption != "srtp" {
		t.Fatalf("media_encryption: got %q want srtp", created.MediaEncryption)
	}

	var fetched gateway.Gateway
	if err := db.WithCtx(ctx, pool, superCtx(), func(tx pgx.Tx) error {
		g2, err := repo.GetTx(ctx, tx, created.ID)
		fetched = g2
		return err
	}); err != nil {
		t.Fatalf("get: %v", err)
	}
	if fetched.MediaEncryption != "srtp" {
		t.Fatalf("fetched media_encryption: got %q want srtp", fetched.MediaEncryption)
	}
}

func TestMediaEncryptionDefaultsNone(t *testing.T) {
	pool := testutil.TestPool(t)
	repo := gateway.NewRepo(testEncKey)
	ctx := context.Background()

	var created gateway.Gateway
	if err := db.WithCtx(ctx, pool, superCtx(), func(tx pgx.Tx) error {
		c, err := repo.CreateTx(ctx, tx, makeGateway("media-enc-default"))
		created = c
		return err
	}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.MediaEncryption != "none" {
		t.Fatalf("default media_encryption: got %q want none", created.MediaEncryption)
	}
}

func TestRLSTenantCannotSeeGateways(t *testing.T) {
	pool := testutil.TestPool(t)
	repo := gateway.NewRepo(testEncKey)
	ctx := context.Background()

	// super_admin creates one
	if err := db.WithCtx(ctx, pool, superCtx(), func(tx pgx.Tx) error {
		_, err := repo.CreateTx(ctx, tx, makeGateway("rls-test"))
		return err
	}); err != nil {
		t.Fatalf("create: %v", err)
	}

	// tenant_owner cannot list
	tenantCtx := db.Ctx{Role: "tenant_owner", TenantID: 1}
	if err := db.WithCtx(ctx, pool, tenantCtx, func(tx pgx.Tx) error {
		list, err := repo.ListTx(ctx, tx)
		if err != nil {
			return err
		}
		if len(list) != 0 {
			t.Fatalf("tenant_owner should see 0 gateways, got %d", len(list))
		}
		return nil
	}); err != nil {
		t.Fatalf("tenant list: %v", err)
	}
}

func TestDisableActiveGatewayAutoDeactivates(t *testing.T) {
	pool := testutil.TestPool(t)
	repo := gateway.NewRepo(testEncKey)
	ctx := context.Background()

	var id int64
	if err := db.WithCtx(ctx, pool, superCtx(), func(tx pgx.Tx) error {
		c, err := repo.CreateTx(ctx, tx, makeGateway("disable-active"))
		id = c.ID
		return err
	}); err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := db.WithCtx(ctx, pool, superCtx(), func(tx pgx.Tx) error {
		return repo.ActivateTx(ctx, tx, id)
	}); err != nil {
		t.Fatalf("activate: %v", err)
	}

	var updated gateway.Gateway
	if err := db.WithCtx(ctx, pool, superCtx(), func(tx pgx.Tx) error {
		var err error
		updated, err = repo.UpdateTx(ctx, tx, id, gateway.UpdatePatch{Enabled: ptr(false)})
		return err
	}); err != nil {
		t.Fatalf("update enabled=false on active gateway: %v", err)
	}

	if updated.Enabled {
		t.Fatal("enabled should be false")
	}
	if updated.IsActive {
		t.Fatal("is_active should be false after disabling")
	}
}

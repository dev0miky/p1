package api

import (
	"crypto/subtle"
	"net/http"

	"github.com/jackc/pgx/v5"

	"p1/engine/internal/db"
	"p1/engine/internal/fsxml"
	"p1/engine/internal/gateway"
)

type fsXML struct {
	repo   *gateway.Repo
	pool   *db.Pool
	secret string
}

func (f *fsXML) handle(w http.ResponseWriter, r *http.Request) {
	if f.secret == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var provided string
	if _, pw, ok := r.BasicAuth(); ok {
		provided = pw
	} else if h := r.Header.Get("X-FS-Secret"); h != "" {
		provided = h
	}

	if subtle.ConstantTimeCompare([]byte(provided), []byte(f.secret)) != 1 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if err := r.ParseForm(); err != nil {
		w.Header().Set("Content-Type", "text/xml; charset=utf-8")
		_, _ = w.Write([]byte(fsxml.NotFound()))
		return
	}

	section := r.FormValue("section")
	keyValue := r.FormValue("key_value")

	if section != "configuration" || keyValue != "sofia.conf" {
		w.Header().Set("Content-Type", "text/xml; charset=utf-8")
		_, _ = w.Write([]byte(fsxml.NotFound()))
		return
	}

	ctx := r.Context()
	var gws []gateway.Gateway
	err := db.WithCtx(ctx, f.pool, db.Ctx{Role: "super_admin"}, func(tx pgx.Tx) error {
		var err error
		gws, err = f.repo.ListEnabledWithSecretTx(ctx, tx)
		return err
	})
	if err != nil {
		w.Header().Set("Content-Type", "text/xml; charset=utf-8")
		_, _ = w.Write([]byte(fsxml.NotFound()))
		return
	}

	views := make([]fsxml.GatewayView, 0, len(gws))
	for _, g := range gws {
		v := fsxml.GatewayView{
			Name:            g.Name,
			Proxy:           g.Proxy,
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
		views = append(views, v)
	}

	out, err := fsxml.RenderSofia(views)
	if err != nil {
		w.Header().Set("Content-Type", "text/xml; charset=utf-8")
		_, _ = w.Write([]byte(fsxml.NotFound()))
		return
	}

	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	_, _ = w.Write([]byte(out))
}

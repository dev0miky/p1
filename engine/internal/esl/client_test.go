package esl_test

import (
	"context"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"p1/engine/internal/esl"
)

func skipIfNoESL(t *testing.T) string {
	t.Helper()
	addr := os.Getenv("TEST_FREESWITCH_ESL")
	if addr == "" {
		t.Skip("TEST_FREESWITCH_ESL not set (e.g. localhost:8021)")
	}
	return addr
}

func testClient(t *testing.T) *esl.Client {
	t.Helper()
	addr := skipIfNoESL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	password := os.Getenv("TEST_FREESWITCH_PASSWORD")
	if password == "" {
		password = "ClueCon"
	}
	c, err := esl.Dial(ctx, addr, password, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(c.Close)
	return c
}

func TestStatusCommand(t *testing.T) {
	c := testClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := c.API(ctx, "status")
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("status returned empty body")
	}
}

func TestSubscribeAndReceiveEvents(t *testing.T) {
	c := testClient(t)
	got := make(chan esl.Event, 32)
	c.OnEvent(func(e esl.Event) {
		select {
		case got <- e:
		default:
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.Subscribe(ctx, "HEARTBEAT", "BACKGROUND_JOB"); err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	if _, err := c.BgAPI(ctx, "status"); err != nil {
		t.Fatalf("bgapi: %v", err)
	}

	deadline := time.After(5 * time.Second)
	for {
		select {
		case e := <-got:
			if e.Name == "BACKGROUND_JOB" {
				return
			}
		case <-deadline:
			t.Fatal("did not receive BACKGROUND_JOB event in time")
		}
	}
}

func TestBgAPIReturnsJobUUID(t *testing.T) {
	c := testClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	jobUUID, err := c.BgAPI(ctx, "status")
	if err != nil {
		t.Fatalf("bgapi: %v", err)
	}
	if jobUUID == "" {
		t.Fatal("expected Job-UUID, got empty")
	}
}

func TestOriginateRejectsEmptyGateway(t *testing.T) {
	c := testClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := c.Originate(ctx, esl.OriginateParams{Dest: "+15551234567"})
	if err == nil {
		t.Fatal("expected error for missing gateway")
	}
}

func TestOriginateRejectsEmptyDest(t *testing.T) {
	c := testClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := c.Originate(ctx, esl.OriginateParams{Gateway: "voxtelesys"})
	if err == nil {
		t.Fatal("expected error for missing dest")
	}
}

package cloudhost_test

import (
	"context"
	"errors"
	"testing"

	"github.com/MajestaNet/ide/sdk/aws/cloudhost"
)

func TestECSHostConfiguredAndResolveBinding(t *testing.T) {
	h := cloudhost.NewECSHost(cloudhost.Config{
		Region:        "us-east-1",
		Cluster:       "one",
		APIService:    "one-api",
		WorkerService: "one-worker",
		RDSInstanceID: "one-pg",
	})
	if h.Host() != "aws" {
		t.Fatalf("host: %q", h.Host())
	}
	if !h.Configured() {
		t.Fatal("expected configured")
	}
	ok, err := h.AccountOK(context.Background())
	if err != nil || !ok {
		t.Fatalf("AccountOK: ok=%v err=%v", ok, err)
	}
	b, err := h.ResolveBinding(context.Background(), cloudhost.BindInput{})
	if err != nil {
		t.Fatal(err)
	}
	if b.AppResourceID != "one/one-api" || b.DatabaseResourceID != "one-pg" {
		t.Fatalf("binding: %+v", b)
	}
	sum, err := h.Describe(context.Background(), b.AppResourceID)
	if err != nil || sum.AppResourceID == "" {
		t.Fatalf("describe: %+v err=%v", sum, err)
	}
	_, err = h.Scale(context.Background(), b.AppResourceID, cloudhost.ScaleInput{})
	if !errors.Is(err, cloudhost.ErrNotImplemented) {
		t.Fatalf("expected ErrNotImplemented, got %v", err)
	}
}

func TestECSHostNotConfigured(t *testing.T) {
	h := cloudhost.NewECSHost(cloudhost.Config{})
	if h.Configured() {
		t.Fatal("expected not configured")
	}
	_, err := h.AccountOK(context.Background())
	if !errors.Is(err, cloudhost.ErrNotConfigured) {
		t.Fatalf("expected ErrNotConfigured, got %v", err)
	}
}

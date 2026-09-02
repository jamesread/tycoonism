package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSetConfigDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("server:\n  listen_addr: \":9999\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	SetConfigDir(dir)
	t.Cleanup(func() { SetConfigDir("") })

	got := GetConfigPath()
	want := filepath.Join(dir, "config.yaml")
	if got != want {
		t.Fatalf("GetConfigPath() = %q, want %q", got, want)
	}
}

func TestListenAddr(t *testing.T) {
	t.Setenv("PORT", "")
	cfg := &Config{ListenAddr: ":9090"}
	if got := ListenAddr(cfg); got != ":9090" {
		t.Fatalf("fallback got %s", got)
	}
	t.Setenv("PORT", "3000")
	if got := ListenAddr(cfg); got != ":3000" {
		t.Fatalf("PORT number got %s", got)
	}
	t.Setenv("PORT", "127.0.0.1:4000")
	if got := ListenAddr(cfg); got != "127.0.0.1:4000" {
		t.Fatalf("PORT addr got %s", got)
	}
	t.Setenv("PORT", "")
	if got := ListenAddr(nil); got != DefaultListenAddr {
		t.Fatalf("nil cfg got %s", got)
	}
}

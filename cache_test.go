package main

import (
	"testing"
	"time"
)

func TestCachePersistsAcrossRestart(t *testing.T) {
	dir := t.TempDir()
	first, err := newCache(dir)
	if err != nil {
		t.Fatal(err)
	}
	first.set("mensa-essen:2026-07-27", []byte("menu"), time.Hour)

	second, err := newCache(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := second.get("mensa-essen:2026-07-27"); !ok || string(got) != "menu" {
		t.Fatalf("restored cache entry = %q, %t; want menu, true", got, ok)
	}
}

package main

import "testing"

func TestVersionDefault(t *testing.T) {
	if version == "" {
		t.Fatal("version must never be empty; default is \"dev\"")
	}
}

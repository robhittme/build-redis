package main

import "testing"

func TestSet(t *testing.T) {
	r := NewRedisStore()
	r.Set("foo", StoredValue{value: "bar"})
	if r.data["foo"].value != "bar" {
		t.Error("Expected bar, got", r.data["foo"].value)
	}
}

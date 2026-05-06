package main

import "testing"

func TestVarMap_Set_Valid(t *testing.T) {
	v := &varMap{}
	if err := v.Set("FOO=bar"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.values["FOO"] != "bar" {
		t.Errorf("expected 'bar', got %q", v.values["FOO"])
	}
}

func TestVarMap_Set_Multiple(t *testing.T) {
	v := &varMap{}
	v.Set("A=1")
	v.Set("B=2")
	if len(v.values) != 2 {
		t.Fatalf("expected 2 values, got %d", len(v.values))
	}
	if v.values["A"] != "1" || v.values["B"] != "2" {
		t.Errorf("unexpected values: %v", v.values)
	}
}

func TestVarMap_Set_Invalid(t *testing.T) {
	v := &varMap{}
	if err := v.Set("NOEQUALSSIGN"); err == nil {
		t.Fatal("expected error for invalid format")
	}
}

func TestVarMap_Set_ValueWithEquals(t *testing.T) {
	v := &varMap{}
	if err := v.Set("URL=http://host?a=b"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.values["URL"] != "http://host?a=b" {
		t.Errorf("expected 'http://host?a=b', got %q", v.values["URL"])
	}
}

func TestStringSlice_Set(t *testing.T) {
	s := &stringSlice{}
	s.Set("alpha")
	s.Set("beta")
	if len(s.values) != 2 {
		t.Fatalf("expected 2 values, got %d", len(s.values))
	}
	if s.values[0] != "alpha" || s.values[1] != "beta" {
		t.Errorf("unexpected values: %v", s.values)
	}
}

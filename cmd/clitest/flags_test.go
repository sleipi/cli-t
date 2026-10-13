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

func TestVarMap_Type(t *testing.T) {
	v := &varMap{}
	if v.Type() != "NAME=VALUE" {
		t.Errorf("expected 'NAME=VALUE', got %q", v.Type())
	}
}

func TestParseReport_Junit(t *testing.T) {
	format, path, err := parseReport("junit:build/report.xml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if format != "junit" || path != "build/report.xml" {
		t.Errorf("got format %q, path %q", format, path)
	}
}

func TestParseReport_PathWithColon(t *testing.T) {
	_, path, err := parseReport("junit:C:/reports/out.xml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != "C:/reports/out.xml" {
		t.Errorf("got path %q", path)
	}
}

func TestParseReport_Invalid(t *testing.T) {
	for _, s := range []string{"junit", "junit:", "tap:out.tap", ":out.xml"} {
		if _, _, err := parseReport(s); err == nil {
			t.Errorf("expected error for %q", s)
		}
	}
}

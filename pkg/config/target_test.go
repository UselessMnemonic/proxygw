package config

import "testing"

func TestTargetEndpointReferenceTextRoundTrip(t *testing.T) {
	t.Parallel()

	ref := TargetEndpointReference{
		TargetName:   "backend",
		EndpointName: "https",
	}

	text, err := ref.MarshalText()
	if err != nil {
		t.Fatalf("MarshalText() error = %v", err)
	}
	if got, want := string(text), "backend:https"; got != want {
		t.Fatalf("MarshalText() = %q, want %q", got, want)
	}

	var parsed TargetEndpointReference
	if err := parsed.UnmarshalText(text); err != nil {
		t.Fatalf("UnmarshalText() error = %v", err)
	}
	if parsed != ref {
		t.Fatalf("round-trip = %#v, want %#v", parsed, ref)
	}
}

func TestTargetEndpointReferenceUnmarshalTextRejectsInvalidFormat(t *testing.T) {
	t.Parallel()

	var ref TargetEndpointReference
	if err := ref.UnmarshalText([]byte("backend/https")); err == nil {
		t.Fatal("UnmarshalText() error = nil, want error")
	}
}

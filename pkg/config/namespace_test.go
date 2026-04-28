package config

import "testing"

func TestNamespaceReferenceTextRoundTrip(t *testing.T) {
	t.Parallel()

	ref := NamespaceReference{
		Namespace: "static",
		Name:      "drop",
	}

	text, err := ref.MarshalText()
	if err != nil {
		t.Fatalf("MarshalText() error = %v", err)
	}
	if got, want := string(text), "static:drop"; got != want {
		t.Fatalf("MarshalText() = %q, want %q", got, want)
	}

	parsed, err := ParseNamespaceReference(string(text))
	if err != nil {
		t.Fatalf("ParseNamespaceReference() error = %v", err)
	}
	if parsed != ref {
		t.Fatalf("round-trip = %#v, want %#v", parsed, ref)
	}
}

func TestNamespaceReferenceRejectsInvalidFormat(t *testing.T) {
	t.Parallel()

	tests := []string{
		"",
		"static",
		"static/drop",
		"static:",
		":drop",
		"static:drop:extra",
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc, func(t *testing.T) {
			t.Parallel()

			if _, err := ParseNamespaceReference(tc); err == nil {
				t.Fatalf("ParseNamespaceReference(%q) error = nil, want error", tc)
			}
		})
	}
}

package config

import (
	"fmt"
	"strings"
)

// NamespaceReference identifies a namespaced object in namespace:name form.
type NamespaceReference struct {
	Namespace string
	Name      string
}

func ParseNamespaceReference(text string) (NamespaceReference, error) {
	parts := strings.Split(text, ":")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return NamespaceReference{}, fmt.Errorf("invalid namespace reference: %q", text)
	}
	return NamespaceReference{
		Namespace: parts[0],
		Name:      parts[1],
	}, nil
}

func (r *NamespaceReference) String() string {
	return fmt.Sprintf("%s:%s", r.Namespace, r.Name)
}

func (r *NamespaceReference) MarshalText() ([]byte, error) {
	return []byte(r.String()), nil
}

func (r *NamespaceReference) UnmarshalText(text []byte) error {
	ref, err := ParseNamespaceReference(string(text))
	if err != nil {
		return err
	}
	*r = ref
	return nil
}

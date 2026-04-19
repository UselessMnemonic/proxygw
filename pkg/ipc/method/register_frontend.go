package method

import "proxygw/pkg/config"

// RegisterFrontendRequest creates a frontend driver instance in the plugin.
type RegisterFrontendRequest struct {
	Kind     string            `json:"kind"`
	Protocol config.Protocol   `json:"protocol"`
	Listen   string            `json:"listen"`
	Options  map[string]any    `json:"options,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// RegisterFrontendResponse returns the opaque frontend handle.
type RegisterFrontendResponse struct {
	FrontendID string `json:"frontend_id"`
}

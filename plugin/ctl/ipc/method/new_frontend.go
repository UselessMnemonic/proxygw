package method

import (
	"proxygw/pkg/config"
)

// NewFrontendRequest asks the plugin to construct a frontend driver instance.
type NewFrontendRequest struct {
	Name     string          `json:"name"`
	Kind     string          `json:"kind"`
	Protocol config.Protocol `json:"protocol"`
	Listen   string          `json:"listen"`
	Options  map[string]any  `json:"options,omitempty"`
}

func (NewFrontendRequest) Method() uint16 {
	return MethodNewFrontendRequest
}

// NewFrontendResponse confirms the frontend driver was created.
type NewFrontendResponse struct{}

func (NewFrontendResponse) Method() uint16 {
	return MethodNewFrontendResponse
}

package method

import "proxygw/pkg/config"

// NewFrontendRequest asks the plugin to construct a frontend driver instance.
type NewFrontendRequest struct {
	FrontendID string          `json:"frontend_id"`
	Kind       string          `json:"kind"`
	Protocol   config.Protocol `json:"protocol"`
	Listen     string          `json:"listen"`
	Options    map[string]any  `json:"options,omitempty"`
}

func (NewFrontendRequest) Method() uint16 {
	return MethodNewFrontendRequest
}

type NewFrontendResponse struct{}

func (NewFrontendResponse) Method() uint16 {
	return MethodNewFrontendResponse
}

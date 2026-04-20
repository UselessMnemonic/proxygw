package method

import "proxygw/pkg/config"

// StatusRequest asks the proxy engine for status details.
type StatusRequest struct{}

func (StatusRequest) Method() uint16 {
	return MethodStatusRequest
}

// StatusResponse reports proxy engine state.
type StatusResponse struct {
	Closed    bool             `json:"closed"`
	Targets   []TargetStatus   `json:"targets,omitempty"`
	Frontends []FrontendStatus `json:"frontends,omitempty"`
}

func (StatusResponse) Method() uint16 {
	return MethodStatusResponse
}

type TargetStatus struct {
	Name      string               `json:"name"`
	Kind      string               `json:"kind"`
	State     string               `json:"state"`
	LastError string               `json:"last_error,omitempty"`
	Endpoints []TargetEndpointInfo `json:"endpoints,omitempty"`
}

type TargetEndpointInfo struct {
	Name     string          `json:"name"`
	Protocol config.Protocol `json:"protocol"`
	Address  string          `json:"address"`
}

type FrontendStatus struct {
	Name         string          `json:"name"`
	Kind         string          `json:"kind"`
	State        string          `json:"state"`
	LastError    string          `json:"last_error,omitempty"`
	Protocol     config.Protocol `json:"protocol"`
	Listen       string          `json:"listen"`
	TargetName   string          `json:"target_name"`
	EndpointName string          `json:"endpoint_name"`
	ProxyAddress string          `json:"proxyaddress"`
}

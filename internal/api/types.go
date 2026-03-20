package api

// PublicConfigResponse 是 /api/v1/public-config 的响应体。
type PublicConfigResponse struct {
	RefreshInterval        string `json:"refresh_interval"`
	RefreshIntervalSeconds int    `json:"refresh_interval_seconds"`
	ShowQueryClients       bool   `json:"show_query_clients"`
	ServerHost             string `json:"server_host"`
	ServerPort             int    `json:"server_port"`
}

// APIError 是错误响应体。
type APIError struct {
	Error string `json:"error"`
}

// HealthResponse 是健康检查响应体。
type HealthResponse struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

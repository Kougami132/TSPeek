package api

// BrandingResponse 是品牌配置的响应体。
type BrandingResponse struct {
	FaviconURL  string `json:"favicon_url"`
	SiteTitle   string `json:"site_title"`
	LogoURL     string `json:"logo_url"`
	HeaderTitle string `json:"header_title"`
}

// WebClientResponse 描述可选的网页版 TeamSpeak 客户端入口。
// URL 为空表示未启用，前端据此隐藏对应按钮。
type WebClientResponse struct {
	URL string `json:"url"`
}

// PublicConfigResponse 是 /api/v1/public-config 的响应体。
type PublicConfigResponse struct {
	ServerHost string            `json:"server_host"`
	ServerPort int               `json:"server_port"`
	Branding   BrandingResponse  `json:"branding"`
	WebClient  WebClientResponse `json:"web_client"`
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

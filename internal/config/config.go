package config

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type fileConfig struct {
	ListenAddress string          `yaml:"listen_address"`
	LogLevel      string          `yaml:"log_level"`
	HTTP          fileHTTPConfig  `yaml:"http"`
	Dashboard     fileDashboard   `yaml:"dashboard"`
	ServerQuery   fileServerQuery `yaml:"serverquery"`
}

type fileHTTPConfig struct {
	ReadTimeout  string `yaml:"read_timeout"`
	WriteTimeout string `yaml:"write_timeout"`
	IdleTimeout  string `yaml:"idle_timeout"`
}

type fileDashboard struct {
	RefreshInterval  string `yaml:"refresh_interval"`
	ShowQueryClients bool   `yaml:"show_query_clients"`
}

type fileServerQuery struct {
	Host           string `yaml:"host"`
	QueryPort      int    `yaml:"query_port"`
	Username       string `yaml:"username"`
	Password       string `yaml:"password"`
	ServerPort     int    `yaml:"server_port"`
	ServerID       int    `yaml:"sid"`
	DialTimeout    string `yaml:"dial_timeout"`
	CommandTimeout string `yaml:"command_timeout"`
}

// Config 是运行时配置，由 Load 函数从 YAML 文件解析并校验后生成。
type Config struct {
	ListenAddress string
	LogLevel      slog.Level
	HTTP          HTTPConfig
	Dashboard     DashboardConfig
	ServerQuery   ServerQueryConfig
}

type HTTPConfig struct {
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

type DashboardConfig struct {
	RefreshInterval  time.Duration
	ShowQueryClients bool
}

type ServerQueryConfig struct {
	Host           string
	QueryPort      int
	Username       string
	Password       string
	ServerPort     int
	ServerID       int
	DialTimeout    time.Duration
	CommandTimeout time.Duration
}

// Load 从指定路径加载 YAML 配置文件并返回校验后的运行时配置。
func Load(path string) (Config, error) {
	rawBytes, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}

	var raw fileConfig
	if err := yaml.Unmarshal(rawBytes, &raw); err != nil {
		return Config{}, err
	}

	refreshInterval, err := parseDuration(raw.Dashboard.RefreshInterval, 5*time.Second)
	if err != nil {
		return Config{}, fmt.Errorf("dashboard.refresh_interval: %w", err)
	}
	if refreshInterval < time.Second {
		return Config{}, fmt.Errorf("dashboard.refresh_interval must be at least 1s")
	}

	readTimeout, err := parseDuration(raw.HTTP.ReadTimeout, 5*time.Second)
	if err != nil {
		return Config{}, fmt.Errorf("http.read_timeout: %w", err)
	}
	writeTimeout, err := parseDuration(raw.HTTP.WriteTimeout, 30*time.Second)
	if err != nil {
		return Config{}, fmt.Errorf("http.write_timeout: %w", err)
	}
	idleTimeout, err := parseDuration(raw.HTTP.IdleTimeout, 60*time.Second)
	if err != nil {
		return Config{}, fmt.Errorf("http.idle_timeout: %w", err)
	}
	dialTimeout, err := parseDuration(raw.ServerQuery.DialTimeout, 5*time.Second)
	if err != nil {
		return Config{}, fmt.Errorf("serverquery.dial_timeout: %w", err)
	}
	commandTimeout, err := parseDuration(raw.ServerQuery.CommandTimeout, 10*time.Second)
	if err != nil {
		return Config{}, fmt.Errorf("serverquery.command_timeout: %w", err)
	}

	if strings.TrimSpace(raw.ServerQuery.Host) == "" {
		return Config{}, fmt.Errorf("serverquery.host is required")
	}
	if raw.ServerQuery.QueryPort == 0 {
		raw.ServerQuery.QueryPort = 10011
	}
	if strings.TrimSpace(raw.ServerQuery.Username) == "" {
		return Config{}, fmt.Errorf("serverquery.username is required")
	}
	if raw.ServerQuery.Password == "" {
		return Config{}, fmt.Errorf("serverquery.password is required")
	}
	if raw.ServerQuery.ServerID <= 0 && raw.ServerQuery.ServerPort <= 0 {
		return Config{}, fmt.Errorf("one of serverquery.sid or serverquery.server_port is required")
	}

	level := slog.LevelInfo
	switch strings.ToLower(strings.TrimSpace(raw.LogLevel)) {
	case "", "info":
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		return Config{}, fmt.Errorf("unsupported log_level %q", raw.LogLevel)
	}

	listenAddress := raw.ListenAddress
	if listenAddress == "" {
		listenAddress = ":8080"
	}

	return Config{
		ListenAddress: listenAddress,
		LogLevel:      level,
		HTTP: HTTPConfig{
			ReadTimeout:  readTimeout,
			WriteTimeout: writeTimeout,
			IdleTimeout:  idleTimeout,
		},
		Dashboard: DashboardConfig{
			RefreshInterval:  refreshInterval,
			ShowQueryClients: raw.Dashboard.ShowQueryClients,
		},
		ServerQuery: ServerQueryConfig{
			Host:           strings.TrimSpace(raw.ServerQuery.Host),
			QueryPort:      raw.ServerQuery.QueryPort,
			Username:       raw.ServerQuery.Username,
			Password:       raw.ServerQuery.Password,
			ServerPort:     raw.ServerQuery.ServerPort,
			ServerID:       raw.ServerQuery.ServerID,
			DialTimeout:    dialTimeout,
			CommandTimeout: commandTimeout,
		},
	}, nil
}

func parseDuration(raw string, fallback time.Duration) (time.Duration, error) {
	if strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(strings.TrimSpace(raw))
	if err != nil {
		return 0, err
	}
	return parsed, nil
}

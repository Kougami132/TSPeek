package config

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type fileConfig struct {
	Port        int             `yaml:"port"`
	LogLevel    string          `yaml:"log_level"`
	ServerQuery fileServerQuery `yaml:"serverquery"`
}

type fileServerQuery struct {
	Host       string `yaml:"host"`
	QueryPort  int    `yaml:"query_port"`
	Username   string `yaml:"username"`
	Password   string `yaml:"password"`
	ServerPort int    `yaml:"server_port"`
}

// Config 是运行时配置，由 Load 函数从 YAML 文件解析并校验后生成。
type Config struct {
	Port        int
	LogLevel    slog.Level
	ServerQuery ServerQueryConfig
}

type ServerQueryConfig struct {
	Host       string
	QueryPort  int
	Username   string
	Password   string
	ServerPort int
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
	if raw.ServerQuery.ServerPort <= 0 {
		return Config{}, fmt.Errorf("serverquery.server_port is required")
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

	port := raw.Port
	if port == 0 {
		port = 8080
	}

	return Config{
		Port:     port,
		LogLevel: level,
		ServerQuery: ServerQueryConfig{
			Host:       strings.TrimSpace(raw.ServerQuery.Host),
			QueryPort:  raw.ServerQuery.QueryPort,
			Username:   raw.ServerQuery.Username,
			Password:   raw.ServerQuery.Password,
			ServerPort: raw.ServerQuery.ServerPort,
		},
	}, nil
}

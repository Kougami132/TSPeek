package icon

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	maxIconSize = 1 * 1024 * 1024
	dialTimeout = 5 * time.Second
	ftTimeout   = 15 * time.Second
	negCacheTTL = 5 * time.Minute
	throttleGap = 500 * time.Millisecond
)

// CommandExecutor 抽象 ServerQuery 指令执行接口。
type CommandExecutor interface {
	Exec(ctx context.Context, cmd string) ([]string, error)
}

// CachedItem 保存已下载图标的二进制内容与 MIME 类型。
type CachedItem struct {
	Body        []byte
	ContentType string
}

// Service 通过共享 ServerQuery 会话发送 ftinitdownload 并通过 FT 端口拉取文件。
type Service struct {
	exec   CommandExecutor
	host   string
	logger *slog.Logger
	cache  sync.Map // map[string]*CachedItem
	neg    sync.Map // map[string]time.Time
	ftMu   sync.Mutex
}

// NewService 创建图标下载服务。
func NewService(exec CommandExecutor, host string, logger *slog.Logger) *Service {
	return &Service{
		exec:   exec,
		host:   host,
		logger: logger,
	}
}

func (s *Service) GetIcon(ctx context.Context, iconID uint32) (*CachedItem, error) {
	// 客户端内置图标（ID < 1000）直接使用内嵌 SVG
	if iconID > 0 && iconID < 1000 {
		return builtinIcon(iconID), nil
	}
	key := fmt.Sprintf("icon_%d", iconID)
	path := fmt.Sprintf("/icon_%d", iconID)
	return s.getFile(ctx, key, path, maxIconSize, negCacheTTL)
}

func (s *Service) getFile(ctx context.Context, cacheKey, ftPath string, maxSize int, negTTL time.Duration) (*CachedItem, error) {
	if v, ok := s.cache.Load(cacheKey); ok {
		return v.(*CachedItem), nil
	}

	if v, ok := s.neg.Load(cacheKey); ok {
		if time.Since(v.(time.Time)) < negTTL {
			return nil, fmt.Errorf("not found (cached)")
		}
		s.neg.Delete(cacheKey)
	}

	s.ftMu.Lock()
	defer s.ftMu.Unlock()

	if v, ok := s.cache.Load(cacheKey); ok {
		return v.(*CachedItem), nil
	}
	if v, ok := s.neg.Load(cacheKey); ok {
		if time.Since(v.(time.Time)) < negTTL {
			return nil, fmt.Errorf("not found (cached)")
		}
	}

	data, err := s.ftDownloadLocked(ctx, ftPath, maxSize)
	if err != nil {
		s.neg.Store(cacheKey, time.Now())
		s.logger.Warn("ft download failed", slog.String("path", ftPath), slog.Any("error", err))
		return nil, err
	}

	item := &CachedItem{
		Body:        data,
		ContentType: http.DetectContentType(data),
	}
	s.cache.Store(cacheKey, item)
	s.logger.Debug("ft download cached", slog.String("key", cacheKey), slog.Int("size", len(data)))
	return item, nil
}

func (s *Service) ftDownloadLocked(ctx context.Context, path string, maxSize int) ([]byte, error) {
	time.Sleep(throttleGap)

	ftCmd := fmt.Sprintf("ftinitdownload clientftfid=1 name=%s cid=0 cpw= seekpos=0 proto=0",
		escapeTS3(path))

	lines, err := s.exec.Exec(ctx, ftCmd)
	if err != nil {
		return nil, fmt.Errorf("ftinitdownload failed: %w", err)
	}

	var ftKey string
	var ftPort, ftSize int

	for _, line := range lines {
		fields := parseFields(line)
		ftKey, ftPort, ftSize = extractFTParams(fields, ftKey, ftPort, ftSize)
	}

	if ftKey == "" || ftPort == 0 || ftSize == 0 {
		return nil, fmt.Errorf("incomplete ft response: key=%q port=%d size=%d", ftKey, ftPort, ftSize)
	}
	if ftSize > maxSize {
		return nil, fmt.Errorf("file too large: %d > %d", ftSize, maxSize)
	}

	dialer := net.Dialer{Timeout: dialTimeout}
	ftAddress := net.JoinHostPort(s.host, strconv.Itoa(ftPort))
	ftConn, err := dialer.DialContext(ctx, "tcp", ftAddress)
	if err != nil {
		return nil, fmt.Errorf("dial ft port: %w", err)
	}
	defer ftConn.Close()

	if err := ftConn.SetDeadline(time.Now().Add(ftTimeout)); err != nil {
		return nil, err
	}
	if _, err := io.WriteString(ftConn, ftKey); err != nil {
		return nil, fmt.Errorf("send ftkey: %w", err)
	}

	data := make([]byte, ftSize)
	if _, err := io.ReadFull(ftConn, data); err != nil {
		return nil, fmt.Errorf("read ft data: %w", err)
	}

	return data, nil
}

func extractFTParams(fields map[string]string, ftKey string, ftPort, ftSize int) (string, int, int) {
	if v, ok := fields["ftkey"]; ok {
		ftKey = v
	}
	if v, ok := fields["port"]; ok {
		ftPort, _ = strconv.Atoi(v)
	}
	if v, ok := fields["size"]; ok {
		ftSize, _ = strconv.Atoi(v)
	}
	return ftKey, ftPort, ftSize
}

func parseFields(line string) map[string]string {
	fields := make(map[string]string)
	for _, token := range strings.Fields(line) {
		key, value, ok := strings.Cut(token, "=")
		if !ok {
			continue
		}
		fields[key] = unescapeTS3(value)
	}
	return fields
}

func escapeTS3(input string) string {
	r := strings.NewReplacer(
		`\`, `\\`,
		`/`, `\/`,
		` `, `\s`,
		`|`, `\p`,
		"\n", `\n`,
		"\r", `\r`,
		"\t", `\t`,
	)
	return r.Replace(input)
}

func unescapeTS3(input string) string {
	r := strings.NewReplacer(
		`\s`, ` `,
		`\p`, `|`,
		`\n`, "\n",
		`\r`, "\r",
		`\t`, "\t",
		`\/`, `/`,
		`\\`, `\`,
	)
	return r.Replace(input)
}

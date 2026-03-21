package icon

import (
	"bufio"
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

	"tspeek/internal/config"
)

const (
	maxIconSize  = 1 * 1024 * 1024
	dialTimeout  = 5 * time.Second
	cmdTimeout   = 10 * time.Second
	ftTimeout    = 15 * time.Second
	negCacheTTL  = 5 * time.Minute
	throttleGap  = 500 * time.Millisecond
)

// CachedItem holds a downloaded file's content and detected MIME type.
type CachedItem struct {
	Body        []byte
	ContentType string
}

// Service downloads icons and avatars via the TS3 File Transfer protocol
// using a single persistent ServerQuery connection with serial downloads.
type Service struct {
	cfg    config.ServerQueryConfig
	logger *slog.Logger
	cache  sync.Map // map[string]*CachedItem
	neg    sync.Map // map[string]time.Time

	// Single global mutex: all FT downloads go through one connection serially.
	ftMu   sync.Mutex
	conn   net.Conn
	reader *bufio.Reader
}

func NewService(cfg config.ServerQueryConfig, logger *slog.Logger) *Service {
	return &Service{
		cfg:    cfg,
		logger: logger,
	}
}

func (s *Service) GetIcon(ctx context.Context, iconID uint32) (*CachedItem, error) {
	// Built-in icons (ID < 1000) are embedded in the TS3 desktop client,
	// not stored on the server filesystem. Serve pre-defined SVGs instead.
	if iconID > 0 && iconID < 1000 {
		return builtinIcon(iconID), nil
	}
	key := fmt.Sprintf("icon_%d", iconID)
	path := fmt.Sprintf("/icon_%d", iconID)
	return s.getFile(ctx, key, path, maxIconSize, negCacheTTL)
}

func (s *Service) getFile(ctx context.Context, cacheKey, ftPath string, maxSize int, negTTL time.Duration) (*CachedItem, error) {
	// Fast path: positive cache
	if v, ok := s.cache.Load(cacheKey); ok {
		return v.(*CachedItem), nil
	}

	// Fast path: negative cache
	if v, ok := s.neg.Load(cacheKey); ok {
		if time.Since(v.(time.Time)) < negTTL {
			return nil, fmt.Errorf("not found (cached)")
		}
		s.neg.Delete(cacheKey)
	}

	// All downloads serialize through one global mutex + one connection
	s.ftMu.Lock()
	defer s.ftMu.Unlock()

	// Re-check caches after acquiring lock (another goroutine may have fetched it)
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

// ensureConnectedLocked establishes and authenticates the persistent SQ connection.
// Must be called with ftMu held.
func (s *Service) ensureConnectedLocked(ctx context.Context) error {
	if s.conn != nil {
		return nil
	}

	address := net.JoinHostPort(s.cfg.Host, strconv.Itoa(s.cfg.QueryPort))
	dialer := net.Dialer{Timeout: dialTimeout}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return fmt.Errorf("dial serverquery: %w", err)
	}

	reader := bufio.NewReader(conn)

	// Read TS3 banner (2 lines)
	if err := conn.SetReadDeadline(time.Now().Add(cmdTimeout)); err != nil {
		conn.Close()
		return err
	}
	if _, err := reader.ReadString('\n'); err != nil {
		conn.Close()
		return fmt.Errorf("read header: %w", err)
	}
	if _, err := reader.ReadString('\n'); err != nil {
		conn.Close()
		return fmt.Errorf("read banner: %w", err)
	}

	// Login
	loginCmd := fmt.Sprintf("login client_login_name=%s client_login_password=%s\n",
		escapeTS3(s.cfg.Username), escapeTS3(s.cfg.Password))
	if err := sendAndCheck(conn, reader, loginCmd); err != nil {
		conn.Close()
		return fmt.Errorf("login: %w", err)
	}

	// Select virtual server
	useCmd := fmt.Sprintf("use port=%d\n", s.cfg.ServerPort)
	if err := sendAndCheck(conn, reader, useCmd); err != nil {
		conn.Close()
		return fmt.Errorf("use: %w", err)
	}

	s.conn = conn
	s.reader = reader
	s.logger.Debug("icon ft connection established")
	return nil
}

// closeLocked tears down the persistent connection. Must be called with ftMu held.
func (s *Service) closeLocked() {
	if s.conn != nil {
		s.conn.Close()
	}
	s.conn = nil
	s.reader = nil
}

// ftDownloadLocked performs a single ftinitdownload + FT transfer.
// Must be called with ftMu held. Reuses the persistent SQ connection.
func (s *Service) ftDownloadLocked(ctx context.Context, path string, maxSize int) ([]byte, error) {
	// Throttle: avoid rapid-fire commands on the same connection
	time.Sleep(throttleGap)

	if err := s.ensureConnectedLocked(ctx); err != nil {
		return nil, err
	}

	// ftinitdownload
	ftCmd := fmt.Sprintf("ftinitdownload clientftfid=1 name=%s cid=0 cpw= seekpos=0 proto=0\n",
		escapeTS3(path))
	if err := s.conn.SetWriteDeadline(time.Now().Add(cmdTimeout)); err != nil {
		s.closeLocked()
		return nil, err
	}
	if _, err := io.WriteString(s.conn, ftCmd); err != nil {
		s.closeLocked()
		return nil, fmt.Errorf("write ftinitdownload: %w", err)
	}

	if err := s.conn.SetReadDeadline(time.Now().Add(cmdTimeout)); err != nil {
		s.closeLocked()
		return nil, err
	}

	var ftKey string
	var ftPort, ftSize int

	for {
		line, err := s.reader.ReadString('\n')
		if err != nil {
			s.closeLocked()
			return nil, fmt.Errorf("read ft response: %w", err)
		}
		line = strings.Trim(line, "\r\n")
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "notify") {
			fields := parseFields(line)
			ftKey, ftPort, ftSize = extractFTParams(fields, ftKey, ftPort, ftSize)
			continue
		}
		if strings.HasPrefix(line, "error ") {
			fields := parseFields(line)
			errID, _ := strconv.Atoi(fields["id"])
			if errID != 0 {
				// Non-fatal for the connection: the SQ session is still valid
				return nil, fmt.Errorf("ft error id=%d: %s", errID, fields["msg"])
			}
			break
		}
		// Data line without recognized prefix
		fields := parseFields(line)
		ftKey, ftPort, ftSize = extractFTParams(fields, ftKey, ftPort, ftSize)
	}

	if ftKey == "" || ftPort == 0 || ftSize == 0 {
		return nil, fmt.Errorf("incomplete ft response: key=%q port=%d size=%d", ftKey, ftPort, ftSize)
	}
	if ftSize > maxSize {
		return nil, fmt.Errorf("file too large: %d > %d", ftSize, maxSize)
	}

	// Connect to FT port and download the binary data
	dialer := net.Dialer{Timeout: dialTimeout}
	ftAddress := net.JoinHostPort(s.cfg.Host, strconv.Itoa(ftPort))
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

func sendAndCheck(conn net.Conn, reader *bufio.Reader, cmd string) error {
	if err := conn.SetWriteDeadline(time.Now().Add(cmdTimeout)); err != nil {
		return err
	}
	if _, err := io.WriteString(conn, cmd); err != nil {
		return err
	}
	if err := conn.SetReadDeadline(time.Now().Add(cmdTimeout)); err != nil {
		return err
	}
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		line = strings.Trim(line, "\r\n")
		if line == "" || strings.HasPrefix(line, "notify") {
			continue
		}
		if strings.HasPrefix(line, "error ") {
			fields := parseFields(line)
			errID, _ := strconv.Atoi(fields["id"])
			if errID != 0 {
				return fmt.Errorf("%s (id=%d)", fields["msg"], errID)
			}
			return nil
		}
	}
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

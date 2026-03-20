package tsquery

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

const (
	dialTimeout    = 5 * time.Second
	commandTimeout = 10 * time.Second
)

func formatCommand(name string, args ...commandArg) string {
	var builder strings.Builder
	builder.WriteString(name)
	for _, arg := range args {
		builder.WriteByte(' ')
		builder.WriteString(arg.Key)
		builder.WriteByte('=')
		builder.WriteString(escapeTS3(arg.Value))
	}
	return builder.String()
}

func (c *Client) ensureConnected(ctx context.Context) error {
	if c.conn != nil {
		return nil
	}

	address := net.JoinHostPort(c.cfg.Host, strconv.Itoa(c.cfg.QueryPort))
	dialer := net.Dialer{Timeout: dialTimeout}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return err
	}

	reader := bufio.NewReader(conn)
	c.conn = conn
	c.reader = reader

	if _, err := c.readLine(); err != nil {
		c.closeLocked()
		return fmt.Errorf("failed to read serverquery header: %w", err)
	}
	if _, err := c.readLine(); err != nil {
		c.closeLocked()
		return fmt.Errorf("failed to read serverquery banner: %w", err)
	}

	loginCommand := formatCommand("login",
		commandArg{Key: "client_login_name", Value: c.cfg.Username},
		commandArg{Key: "client_login_password", Value: c.cfg.Password},
	)
	if _, err := c.exec(ctx, loginCommand); err != nil {
		c.closeLocked()
		return err
	}

	useCommand := formatCommand("use", commandArg{Key: "port", Value: strconv.Itoa(c.cfg.ServerPort)})
	if _, err := c.exec(ctx, useCommand); err != nil {
		c.closeLocked()
		return err
	}

	return nil
}

func (c *Client) exec(_ context.Context, command string) ([]string, error) {
	if c.conn == nil {
		return nil, fmt.Errorf("serverquery connection is not available")
	}

	if err := c.conn.SetWriteDeadline(time.Now().Add(commandTimeout)); err != nil {
		return nil, err
	}
	if _, err := io.WriteString(c.conn, command+"\n"); err != nil {
		return nil, err
	}

	lines := make([]string, 0, 4)
	for {
		line, err := c.readLine()
		if err != nil {
			return nil, err
		}
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "notify") {
			continue
		}
		if strings.HasPrefix(line, "error ") {
			if err := parseErrorLine(line); err != nil {
				return nil, err
			}
			return lines, nil
		}
		lines = append(lines, line)
	}
}

func (c *Client) readLine() (string, error) {
	if c.conn == nil || c.reader == nil {
		return "", io.EOF
	}
	if err := c.conn.SetReadDeadline(time.Now().Add(commandTimeout)); err != nil {
		return "", err
	}
	line, err := c.reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	// TS3 ServerQuery 使用 \n\r（LF+CR）作为行终止符，
	// ReadString('\n') 在 \n 处分割后，下一行首部会残留 \r，
	// 需要同时去除首尾的 \r\n 字符。
	return strings.Trim(line, "\r\n"), nil
}

func (c *Client) closeLocked() {
	if c.conn != nil {
		_ = c.conn.Close()
	}
	c.conn = nil
	c.reader = nil
}

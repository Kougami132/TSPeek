package tsquery

import (
	"fmt"
	"log/slog"
	"strings"
	"time"
)

const (
	dialTimeout    = 5 * time.Second
	commandTimeout = 10 * time.Second
)

type commandArg struct {
	Key   string
	Value string
}

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

func tryUpdateNickname(baseName string, exec func(string) ([]string, error), logger *slog.Logger) {
	// 先通过 whoami 检查当前昵称，若已经一致则无需发送 clientupdate，避免 TS3 返回 id=513
	whoamiLines, err := exec("whoami")
	if err == nil {
		rows := parseResponseRows(whoamiLines)
		if len(rows) > 0 && rows[0]["client_nickname"] == baseName {
			return
		}
	}

	for i := 0; i < 5; i++ {
		name := baseName
		if i > 0 {
			name = fmt.Sprintf("%s %d", baseName, i)
		}
		cmd := formatCommand("clientupdate", commandArg{Key: "client_nickname", Value: name})
		_, err := exec(cmd)
		if err == nil {
			return
		}
		if !strings.Contains(err.Error(), "nickname is already in use") && !strings.Contains(err.Error(), "id=513") {
			if logger != nil {
				logger.Warn("failed to set query nickname", slog.String("nickname", name), slog.String("error", err.Error()))
			}
			return
		}
	}
	if logger != nil {
		logger.Warn("failed to set query nickname after retries", slog.String("base_nickname", baseName))
	}
}

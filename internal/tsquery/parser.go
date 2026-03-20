package tsquery

import (
	"fmt"
	"strconv"
	"strings"
)

func parseErrorLine(line string) error {
	fields := parseRow(strings.TrimPrefix(line, "error "))
	code := parseInt(fields["id"])
	if code == 0 {
		return nil
	}
	return &QueryError{
		ID:      code,
		Message: fields["msg"],
		Extra:   fields,
	}
}

func parseResponseRows(lines []string) []map[string]string {
	rows := make([]map[string]string, 0)
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		for _, rawRow := range splitEscaped(line, '|') {
			rawRow = strings.TrimSpace(rawRow)
			if rawRow == "" {
				continue
			}
			rows = append(rows, parseRow(rawRow))
		}
	}
	return rows
}

func parseRow(row string) map[string]string {
	fields := make(map[string]string)
	for _, token := range splitEscaped(strings.TrimSpace(row), ' ') {
		if token == "" {
			continue
		}
		key, value, ok := strings.Cut(token, "=")
		if !ok {
			fields[token] = ""
			continue
		}
		fields[key] = unescapeTS3(value)
	}
	return fields
}

func splitEscaped(input string, sep byte) []string {
	if input == "" {
		return nil
	}

	result := make([]string, 0, 8)
	start := 0
	backslashes := 0

	for i := 0; i < len(input); i++ {
		switch input[i] {
		case '\\':
			backslashes++
			continue
		case sep:
			if backslashes%2 == 0 {
				part := input[start:i]
				if sep != ' ' || part != "" {
					result = append(result, part)
				}
				start = i + 1
			}
		}
		backslashes = 0
	}

	if start <= len(input) {
		part := input[start:]
		if sep != ' ' || part != "" {
			result = append(result, part)
		}
	}

	return result
}

func escapeTS3(input string) string {
	var builder strings.Builder
	builder.Grow(len(input))

	for _, r := range input {
		switch r {
		case ' ':
			builder.WriteString(`\s`)
		case '|':
			builder.WriteString(`\p`)
		case '\n':
			builder.WriteString(`\n`)
		case '\r':
			builder.WriteString(`\r`)
		case '\t':
			builder.WriteString(`\t`)
		case '\\':
			builder.WriteString(`\\`)
		case '/':
			builder.WriteString(`\/`)
		default:
			builder.WriteRune(r)
		}
	}

	return builder.String()
}

func unescapeTS3(input string) string {
	if input == "" {
		return ""
	}

	var builder strings.Builder
	builder.Grow(len(input))

	for i := 0; i < len(input); i++ {
		if input[i] != '\\' {
			builder.WriteByte(input[i])
			continue
		}
		if i+1 >= len(input) {
			builder.WriteByte('\\')
			break
		}
		i++
		switch input[i] {
		case 's':
			builder.WriteByte(' ')
		case 'p':
			builder.WriteByte('|')
		case 'n':
			builder.WriteByte('\n')
		case 'r':
			builder.WriteByte('\r')
		case 't':
			builder.WriteByte('\t')
		case '\\':
			builder.WriteByte('\\')
		case '/':
			builder.WriteByte('/')
		default:
			builder.WriteByte(input[i])
		}
	}

	return builder.String()
}

func parseInt(raw string) int {
	if raw == "" {
		return 0
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0
	}
	return value
}

func parseInt64(raw string) int64 {
	if raw == "" {
		return 0
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0
	}
	return value
}

func parseBool(raw string) bool {
	return raw == "1" || strings.EqualFold(raw, "true")
}

// QueryError 表示 ServerQuery 返回的非零错误。
type QueryError struct {
	ID      int
	Message string
	Extra   map[string]string
}

func (e *QueryError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message == "" {
		return fmt.Sprintf("serverquery error id=%d", e.ID)
	}
	return fmt.Sprintf("serverquery error id=%d: %s", e.ID, e.Message)
}

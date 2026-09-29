package activity

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// Storage 管理活动记录在 append-only JSON Lines 文件中的持久化和读取。
type Storage struct {
	path string
	mu   sync.Mutex
}

// NewStorage 创建 Storage 实例。
func NewStorage(path string) *Storage {
	return &Storage{path: path}
}

// Append 以追加模式向日志文件写入一批事件，完成后立即关闭文件句柄。
func (s *Storage) Append(events []Event) error {
	if len(events) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	dir := filepath.Dir(s.path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	f, err := os.OpenFile(s.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	var buf bytes.Buffer
	for _, ev := range events {
		data, err := json.Marshal(ev)
		if err != nil {
			return err
		}
		buf.Write(data)
		buf.WriteByte('\n')
	}

	_, err = f.Write(buf.Bytes())
	return err
}

// ReadPage 从文件尾部反向读取指定页面的活动记录（最新在前），以 O(1) 内存完成分页。
func (s *Storage) ReadPage(page, pageSize int) (PageResult, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 50
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	emptyResult := PageResult{
		Items:      []Event{},
		Page:       page,
		PageSize:   pageSize,
		Total:      0,
		TotalPages: 0,
	}

	f, err := os.Open(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return emptyResult, nil
		}
		return emptyResult, err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return emptyResult, err
	}
	fileSize := fi.Size()
	if fileSize == 0 {
		return emptyResult, nil
	}

	// 1. 统计总有效行数
	totalLines, err := countLines(f, fileSize)
	if err != nil {
		return emptyResult, err
	}
	if totalLines == 0 {
		return emptyResult, nil
	}

	totalPages := (totalLines + pageSize - 1) / pageSize

	skip := (page - 1) * pageSize
	if skip >= totalLines {
		return PageResult{
			Items:      []Event{},
			Page:       page,
			PageSize:   pageSize,
			Total:      totalLines,
			TotalPages: totalPages,
		}, nil
	}

	limit := pageSize
	if skip+limit > totalLines {
		limit = totalLines - skip
	}

	// 2. 从文件尾部反向读取 skip 到 skip+limit 行
	rawLines, err := readLinesReverse(f, fileSize, skip, limit)
	if err != nil {
		return emptyResult, err
	}

	items := make([]Event, 0, len(rawLines))
	for _, raw := range rawLines {
		var ev Event
		if err := json.Unmarshal(raw, &ev); err != nil {
			continue
		}
		items = append(items, ev)
	}

	return PageResult{
		Items:      items,
		Page:       page,
		PageSize:   pageSize,
		Total:      totalLines,
		TotalPages: totalPages,
	}, nil
}

func countLines(f *os.File, size int64) (int, error) {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return 0, err
	}

	buf := make([]byte, 64*1024)
	count := 0
	lastByte := byte('\n')

	for {
		n, err := f.Read(buf)
		if n > 0 {
			count += bytes.Count(buf[:n], []byte{'\n'})
			lastByte = buf[n-1]
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return 0, err
		}
	}

	// 若文件末尾未以换行符结尾，则末行也算一行
	if lastByte != '\n' && size > 0 {
		count++
	}

	return count, nil
}

func readLinesReverse(f *os.File, fileSize int64, skip, limit int) ([][]byte, error) {
	const chunkSize = 8 * 1024
	var result [][]byte
	cursor := fileSize
	var leftover []byte
	linesSeen := 0

	for cursor > 0 && len(result) < limit {
		readSize := int64(chunkSize)
		if cursor < readSize {
			readSize = cursor
		}
		cursor -= readSize

		chunk := make([]byte, readSize)
		if _, err := f.ReadAt(chunk, cursor); err != nil && err != io.EOF {
			return nil, err
		}

		if len(leftover) > 0 {
			chunk = append(chunk, leftover...)
			leftover = nil
		}

		// 从 chunk 右向左切分换行符
		for {
			idx := bytes.LastIndexByte(chunk, '\n')
			if idx == -1 {
				// 未找到换行符，将整段保留为 leftover，继续往前读
				leftover = chunk
				break
			}

			line := chunk[idx+1:]
			chunk = chunk[:idx]

			// 去除末尾 CR 符号
			line = bytes.TrimRight(line, "\r")

			// 跳过文件结尾可能存在的空行
			if len(line) == 0 && cursor == fileSize-readSize && idx == len(chunk) {
				continue
			}
			if len(line) == 0 {
				continue
			}

			if linesSeen < skip {
				linesSeen++
				continue
			}

			result = append(result, line)
			linesSeen++
			if len(result) >= limit {
				break
			}
		}
	}

	// 如果到了文件最开头仍有 leftover 且未达到 limit
	if cursor == 0 && len(leftover) > 0 && len(result) < limit {
		line := bytes.TrimRight(leftover, "\r")
		if len(line) > 0 {
			if linesSeen >= skip {
				result = append(result, line)
			}
		}
	}

	return result, nil
}

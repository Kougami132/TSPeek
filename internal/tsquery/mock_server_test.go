package tsquery_test

import (
	"bufio"
	"net"
	"strings"
	"sync"
	"testing"
)

type mockServer struct {
	t        *testing.T
	listener net.Listener
	port     int

	mu       sync.Mutex
	conn     net.Conn
	writer   *bufio.Writer
	commands []string
	closed   bool

	baselineRows string // response for clientlist -uid
}

func newMockServer(t *testing.T, baselineRows string) *mockServer {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start mock server: %v", err)
	}

	ms := &mockServer{
		t:            t,
		listener:     ln,
		port:         ln.Addr().(*net.TCPAddr).Port,
		baselineRows: baselineRows,
	}

	go ms.acceptLoop()
	return ms
}

func (s *mockServer) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}

		s.mu.Lock()
		s.conn = conn
		s.writer = bufio.NewWriter(conn)
		s.mu.Unlock()

		// Send TS3 welcome banner
		_, _ = conn.Write([]byte("TS3\n\rWelcome to the TeamSpeak 3 ServerQuery interface.\n\r"))

		go s.handleConnection(conn)
	}
}

func (s *mockServer) handleConnection(conn net.Conn) {
	reader := bufio.NewReader(conn)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		trimmed := strings.TrimRight(line, "\r\n")
		if trimmed == "" {
			continue
		}

		s.mu.Lock()
		s.commands = append(s.commands, trimmed)
		writer := s.writer
		s.mu.Unlock()

		if strings.HasPrefix(trimmed, "login ") || strings.HasPrefix(trimmed, "use ") || strings.HasPrefix(trimmed, "servernotifyregister ") {
			s.mu.Lock()
			_, _ = writer.WriteString("error id=0 msg=ok\n\r")
			_ = writer.Flush()
			s.mu.Unlock()
		} else if strings.HasPrefix(trimmed, "clientlist") {
			s.mu.Lock()
			if s.baselineRows != "" {
				_, _ = writer.WriteString(s.baselineRows + "\n\r")
			}
			_, _ = writer.WriteString("error id=0 msg=ok\n\r")
			_ = writer.Flush()
			s.mu.Unlock()
		} else if strings.HasPrefix(trimmed, "whoami") {
			s.mu.Lock()
			_, _ = writer.WriteString("virtualserver_status=online client_id=1\n\rerror id=0 msg=ok\n\r")
			_ = writer.Flush()
			s.mu.Unlock()
		} else {
			s.mu.Lock()
			_, _ = writer.WriteString("error id=0 msg=ok\n\r")
			_ = writer.Flush()
			s.mu.Unlock()
		}
	}
}

func (s *mockServer) SendNotification(line string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.writer != nil {
		_, _ = s.writer.WriteString(line + "\n\r")
		_ = s.writer.Flush()
	}
}

func (s *mockServer) SetBaselineRows(rows string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.baselineRows = rows
}

func (s *mockServer) DisconnectClient() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn != nil {
		_ = s.conn.Close()
		s.conn = nil
		s.writer = nil
	}
}

func (s *mockServer) Close() {
	s.mu.Lock()
	s.closed = true
	if s.conn != nil {
		_ = s.conn.Close()
	}
	s.mu.Unlock()
	_ = s.listener.Close()
}

func (s *mockServer) Commands() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	copied := make([]string, len(s.commands))
	copy(copied, s.commands)
	return copied
}

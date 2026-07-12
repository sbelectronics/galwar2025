package telnetserver

import (
	"net"
	"strings"
	"testing"
	"time"

	"github.com/sbelectronics/galwar/pkg/galwar"
)

type telnetClient struct {
	t    *testing.T
	conn net.Conn
	buf  strings.Builder
}

// expect reads (stripping telnet IAC negotiation bytes) until the output
// contains substr.
func (c *telnetClient) expect(substr string) string {
	c.t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	buf := make([]byte, 4096)
	for {
		if strings.Contains(c.buf.String(), substr) {
			out := c.buf.String()
			c.buf.Reset()
			return out
		}
		c.conn.SetReadDeadline(deadline)
		n, err := c.conn.Read(buf)
		if err != nil {
			c.t.Fatalf("waiting for %q: %v\ngot so far:\n%s", substr, err, c.buf.String())
		}
		// strip IAC option sequences (server sends WILL ECHO / WILL SGA)
		for i := 0; i < n; i++ {
			b := buf[i]
			if b == 255 && i+2 < n {
				i += 2
				continue
			}
			if b >= 32 || b == '\n' || b == '\r' {
				c.buf.WriteByte(b)
			}
		}
	}
}

func (c *telnetClient) sendLine(line string) {
	c.t.Helper()
	if _, err := c.conn.Write([]byte(line + "\r\n")); err != nil {
		c.t.Fatalf("send %q: %v", line, err)
	}
}

func dialTelnet(t *testing.T, addr string) *telnetClient {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	return &telnetClient{t: t, conn: conn}
}

func TestTelnetEndToEnd(t *testing.T) {
	u := galwar.NewUniverse()
	u.Generate(100)
	u.SeedDefaultConfig()
	u.Start()

	srv := New(u)
	srv.IdleTimeout = 30 * time.Second
	if err := srv.Start("127.0.0.1:0"); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer srv.Stop()
	addr := srv.listener.Addr().String()

	// register a new account (declining color keeps expectations literal)
	c := dialTelnet(t, addr)
	c.expect("ANSI color")
	c.sendLine("n")
	c.expect("Handle (or NEW to sign up):")
	c.sendLine("NEW")
	c.expect("Choose your handle:")
	c.sendLine("Telnet Tester")
	c.expect("New password:")
	c.sendLine("opensesame")
	c.expect("Repeat password:")
	c.sendLine("opensesame")
	c.expect("Password set.")
	c.expect("Main Command")
	c.sendLine("i")
	c.expect("Credits: 35000")
	c.sendLine("q")
	c.expect("Goodbye, Telnet Tester!")
	c.conn.Close()

	// wrong password is refused, right password gets in; this session takes
	// the color default (Y) and must still work
	c2 := dialTelnet(t, addr)
	c2.expect("ANSI color")
	c2.sendLine("")
	c2.expect("Handle (or NEW to sign up):")
	c2.sendLine("telnet tester") // case/spacing-insensitive lookup
	c2.expect("Password:")
	c2.sendLine("wrongwrong")
	c2.expect("Wrong password.")
	c2.sendLine("opensesame")
	c2.expect("Main Command")
	c2.sendLine("q")
	c2.expect("Goodbye")
	c2.conn.Close()

	// unknown handle
	c3 := dialTelnet(t, addr)
	c3.expect("ANSI color")
	c3.sendLine("n")
	c3.expect("Handle (or NEW to sign up):")
	c3.sendLine("Nobody Home")
	c3.expect("No such trader")
	c3.conn.Close()
}

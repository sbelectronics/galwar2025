// Package telnetserver is the classic front-end: a TCP listener that speaks
// enough telnet to serve PuTTY, netcat, and real telnet clients, driving the
// same ConsoleUI as the console and the web. Authentication is by handle +
// password, in the spirit of the original BBS door (which trusted the BBS's
// caller identity; we have no BBS, so we have passwords).
package telnetserver

import (
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/sbelectronics/galwar/pkg/consoleui"
	"github.com/sbelectronics/galwar/pkg/galwar"
	"github.com/sbelectronics/galwar/pkg/ratelimit"
)

type Server struct {
	Universe    *galwar.UniverseType
	IdleTimeout time.Duration

	listener net.Listener
	connRate *ratelimit.Keyed // per-IP connection throttle
	mu       sync.Mutex
	conns    map[net.Conn]struct{}
	done     chan struct{}
}

func New(u *galwar.UniverseType) *Server {
	return &Server{
		Universe:    u,
		IdleTimeout: 15 * time.Minute,
		connRate:    ratelimit.NewKeyed(0.2, 10), // ~1 connection / 5s, burst 10, per IP
		conns:       map[net.Conn]struct{}{},
		done:        make(chan struct{}),
	}
}

// Start begins accepting telnet connections on addr (e.g. ":2323").
func (s *Server) Start(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	s.listener = ln
	go s.acceptLoop()
	return nil
}

func (s *Server) acceptLoop() {
	defer close(s.done)
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return // listener closed
		}
		if host, _, herr := net.SplitHostPort(conn.RemoteAddr().String()); herr == nil && !s.connRate.Allow(host) {
			// reject off the accept loop, with a write deadline: a slow or
			// unread client must not stall accepts - and this path fires
			// precisely during a connection flood
			go func(c net.Conn) {
				c.SetWriteDeadline(time.Now().Add(5 * time.Second))
				c.Write([]byte("Too many connections from your address; slow down.\r\n"))
				c.Close()
			}(conn)
			continue
		}
		s.mu.Lock()
		s.conns[conn] = struct{}{}
		s.mu.Unlock()
		go s.serve(conn)
	}
}

// Stop closes the listener and every live connection.
func (s *Server) Stop() {
	if s.listener == nil {
		return
	}
	s.listener.Close()
	s.mu.Lock()
	for conn := range s.conns {
		conn.Close()
	}
	s.mu.Unlock()
	<-s.done
}

func (s *Server) serve(conn net.Conn) {
	defer func() {
		s.mu.Lock()
		delete(s.conns, conn)
		s.mu.Unlock()
		conn.Close()
	}()
	defer func() {
		if r := recover(); r != nil {
			log.Printf("telnet session panic: %v", r)
		}
	}()

	term := newTelnetTerminal(conn, s.IdleTimeout)
	term.negotiate() // IAC WILL ECHO + SGA: char mode, server-side echo

	// the classic BBS-door question (the original detected graphics from
	// the BBS dropfile; a bare socket has to ask)
	term.Printf("\nDo you want ANSI color (Y/n)? ")
	if line, err := term.ReadLine(); err != nil {
		return
	} else {
		answer := strings.ToLower(strings.TrimSpace(line))
		term.color = answer == "" || strings.HasPrefix(answer, "y")
	}

	term.Printf("\n%s=== %sGALACTIC WARZONE%s ===%s\n", consoleui.Cyan, consoleui.White, consoleui.Cyan, consoleui.Reset)
	term.Printf("(telnet gateway - or play at the web portal)\n")

	player := s.authenticate(term)
	if player == nil {
		return
	}

	s.Universe.Do(func() {
		s.Universe.TouchLastSeen(player, time.Now().Unix())
	})

	// reconstruction-after-death and news delivery; a player who died today
	// is turned away until tomorrow
	if !consoleui.SessionStart(s.Universe, term, player) {
		return
	}

	ui := consoleui.NewConsoleUI(s.Universe, player, term)
	ui.Run()
	term.Printf("\nGoodbye, %s!\n", player.GetName())
}

// authenticate resolves a connection to a player: existing handle +
// password, or NEW to register (name moderation applies) and set a
// password. Three failed password attempts end the connection.
func (s *Server) authenticate(term *telnetTerminal) *galwar.Player {
	u := s.Universe
	for attempt := 0; attempt < 5; attempt++ {
		term.Printf("\nHandle (or NEW to sign up): ")
		line, err := term.ReadLine()
		if err != nil {
			return nil
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.EqualFold(line, "new") {
			player := consoleui.RegisterFlow(u, term, "", "")
			if player == nil {
				return nil
			}
			term.Printf("Now set a password for future telnet logins.\n")
			if !s.setPassword(term, player) {
				return nil
			}
			return player
		}

		var player *galwar.Player
		u.Do(func() {
			player = u.Players.GetByNormalizedName(line)
		})
		if player == nil {
			term.Printf("No such trader. Type NEW to sign up.\n")
			continue
		}
		if player.PassHash == "" {
			term.Printf("That account has no telnet password. Log in on the web portal and use the PASS command to set one.\n")
			continue
		}
		for tries := 0; tries < 3; tries++ {
			term.Printf("Password: ")
			pass, err := term.ReadSecret()
			if err != nil {
				return nil
			}
			if player.CheckTelnetPassword(pass) {
				var banned bool
				var name string
				u.Do(func() {
					banned = player.Banned
					name = player.GetName()
				})
				if banned {
					// operational log, not the persisted audit ring, so a
					// banned client can't churn or evict the in-game audit
					log.Printf("blocked login by banned player %q (telnet)", name)
					term.Printf("Your account has been suspended. Contact the sysop.\n")
					return nil
				}
				return player
			}
			term.Printf("Wrong password.\n")
			time.Sleep(time.Second) // slow the brute force
		}
		return nil
	}
	return nil
}

func (s *Server) setPassword(term *telnetTerminal, player *galwar.Player) bool {
	for tries := 0; tries < 5; tries++ {
		term.Printf("New password: ")
		pass, err := term.ReadSecret()
		if err != nil {
			return false
		}
		term.Printf("Repeat password: ")
		again, err := term.ReadSecret()
		if err != nil {
			return false
		}
		if pass != again {
			term.Printf("Passwords do not match.\n")
			continue
		}
		serr := s.Universe.DoErr(func() error {
			return s.Universe.SetTelnetPassword(player, pass)
		})
		if serr != nil {
			term.Printf("%s\n", consoleui.ErrText(serr))
			continue
		}
		term.Printf("Password set.\n")
		return true
	}
	return false
}

// telnet protocol bytes
const (
	tnIAC   = 255
	tnDONT  = 254
	tnDO    = 253
	tnWONT  = 252
	tnWILL  = 251
	tnSB    = 250
	tnSE    = 240
	optEcho = 1
	optSGA  = 3
)

// telnetTerminal implements consoleui.Terminal (and SecretReader) over a raw
// TCP connection: filters IAC negotiation from input, does server-side line
// editing with echo (clients are put in character mode), and converts LF to
// CRLF on output.
type telnetTerminal struct {
	conn        net.Conn
	idleTimeout time.Duration
	inbuf       []byte
	color       bool
	cmdRate     *ratelimit.Bucket
}

func newTelnetTerminal(conn net.Conn, idle time.Duration) *telnetTerminal {
	return &telnetTerminal{conn: conn, idleTimeout: idle, cmdRate: ratelimit.NewBucket(10, 20)}
}

// negotiate puts the client in character-at-a-time mode with server echo.
func (t *telnetTerminal) negotiate() {
	t.conn.Write([]byte{tnIAC, tnWILL, optEcho, tnIAC, tnWILL, optSGA, tnIAC, tnDO, optSGA})
}

func (t *telnetTerminal) Printf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if !t.color {
		msg = consoleui.StripANSI(msg)
	}
	msg = strings.ReplaceAll(msg, "\n", "\r\n")
	t.conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
	t.conn.Write([]byte(msg))
}

// readByte returns the next input byte with IAC sequences filtered out.
func (t *telnetTerminal) readByte() (byte, error) {
	for {
		if len(t.inbuf) == 0 {
			buf := make([]byte, 512)
			t.conn.SetReadDeadline(time.Now().Add(t.idleTimeout))
			n, err := t.conn.Read(buf)
			if err != nil {
				return 0, err
			}
			t.inbuf = buf[:n]
		}
		b := t.inbuf[0]
		t.inbuf = t.inbuf[1:]
		if b != tnIAC {
			return b, nil
		}
		// IAC sequence: swallow it
		cmd, err := t.rawByte()
		if err != nil {
			return 0, err
		}
		switch cmd {
		case tnIAC:
			return tnIAC, nil // escaped literal 255
		case tnWILL, tnWONT, tnDO, tnDONT:
			if _, err := t.rawByte(); err != nil { // option byte
				return 0, err
			}
		case tnSB:
			// subnegotiation: skip until IAC SE
			var prev byte
			for {
				b, err := t.rawByte()
				if err != nil {
					return 0, err
				}
				if prev == tnIAC && b == tnSE {
					break
				}
				prev = b
			}
		default:
			// two-byte command (NOP, etc.) - already consumed
		}
	}
}

// rawByte reads one byte without IAC interpretation (used inside sequences).
func (t *telnetTerminal) rawByte() (byte, error) {
	if len(t.inbuf) == 0 {
		buf := make([]byte, 512)
		t.conn.SetReadDeadline(time.Now().Add(t.idleTimeout))
		n, err := t.conn.Read(buf)
		if err != nil {
			return 0, err
		}
		t.inbuf = buf[:n]
	}
	b := t.inbuf[0]
	t.inbuf = t.inbuf[1:]
	return b, nil
}

// readLineEcho assembles a line with server-side editing. echoChar is what
// to show per keystroke: 0 = the character itself, '*' = masked, -1 = nothing.
func (t *telnetTerminal) readLineEcho(echoChar rune) (string, error) {
	if t.cmdRate != nil {
		// throttle scripted hammering; here (not in ReadLine) so ReadSecret
		// - password entry - is covered too
		t.cmdRate.Wait()
	}
	var line []byte
	for {
		b, err := t.readByte()
		if err != nil {
			return "", err
		}
		switch {
		case b == '\r' || b == '\n':
			// telnet CR may be followed by NUL or LF; the filter loop will
			// simply see and discard it as an empty line next time - but
			// swallow an immediately buffered companion to be tidy
			if len(t.inbuf) > 0 && (t.inbuf[0] == '\n' || t.inbuf[0] == 0) {
				t.inbuf = t.inbuf[1:]
			}
			t.conn.Write([]byte("\r\n"))
			return string(line), nil
		case b == 0x7f || b == 0x08: // backspace
			if len(line) > 0 {
				line = line[:len(line)-1]
				if echoChar >= 0 {
					t.conn.Write([]byte("\b \b"))
				}
			}
		case b >= 32 && b < 127:
			line = append(line, b)
			switch {
			case echoChar == 0:
				t.conn.Write([]byte{b})
			case echoChar > 0:
				t.conn.Write([]byte(string(echoChar)))
			}
		}
	}
}

func (t *telnetTerminal) ReadLine() (string, error) {
	return t.readLineEcho(0)
}

func (t *telnetTerminal) ReadSecret() (string, error) {
	return t.readLineEcho('*')
}

package webserver

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/sbelectronics/galwar/pkg/consoleui"
	"github.com/sbelectronics/galwar/pkg/galwar"
)

// gameSession adapts one WebSocket connection to the consoleui.Terminal
// interface and runs the game UI over it. Output is decoupled through a
// buffered channel with a dedicated writer goroutine, so a slow or stalled
// browser can never block the universe actor - if the buffer fills, the
// session is dropped instead.

type gameSession struct {
	server *Server
	conn   *websocket.Conn
	sub    string
	email  string

	lines     chan string
	out       chan []byte
	closed    chan struct{}
	closeOnce sync.Once
}

func newGameSession(s *Server, conn *websocket.Conn, sub, email string) *gameSession {
	return &gameSession{
		server: s,
		conn:   conn,
		sub:    sub,
		email:  email,
		lines:  make(chan string, 16),
		out:    make(chan []byte, 256),
		closed: make(chan struct{}),
	}
}

func (gs *gameSession) close() {
	gs.closeOnce.Do(func() {
		close(gs.closed)
	})
}

// Printf implements consoleui.Terminal. Never blocks: overflow closes the
// session rather than stalling the caller (which may be the universe actor).
func (gs *gameSession) Printf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	msg = strings.ReplaceAll(msg, "\n", "\r\n") // xterm wants CRLF
	select {
	case gs.out <- []byte(msg):
	case <-gs.closed:
	default:
		// browser isn't draining; cut it loose
		gs.close()
	}
}

// ReadLine implements consoleui.Terminal, with the Tier-0 idle timeout.
func (gs *gameSession) ReadLine() (string, error) {
	idle := time.NewTimer(gs.server.cfg.IdleTimeout)
	defer idle.Stop()
	select {
	case line := <-gs.lines:
		return line, nil
	case <-idle.C:
		gs.Printf("\n\nIdle too long - disconnecting. Your game is saved.\n")
		gs.close()
		return "", fmt.Errorf("idle timeout")
	case <-gs.closed:
		return "", io.EOF
	}
}

// Control frames are out-of-band messages to the browser client, prefixed
// with NUL (which never appears in ANSI game output) so app.js can tell them
// apart from terminal bytes. They travel through the same ordered output
// channel as text, so a control frame lands exactly where it was emitted
// relative to the surrounding prompts.
func (gs *gameSession) sendControl(cmd string) {
	select {
	case gs.out <- []byte("\x00" + cmd):
	case <-gs.closed:
	default:
		gs.close()
	}
}

// ReadSecret implements consoleui.SecretReader: it tells the browser to stop
// local-echoing before reading, so passwords entered on the web are not
// displayed (and can't leak into screenshots or session recordings).
func (gs *gameSession) ReadSecret() (string, error) {
	gs.sendControl("secret:on")
	defer gs.sendControl("secret:off")
	return gs.ReadLine()
}

func (gs *gameSession) reader() {
	for {
		_, msg, err := gs.conn.ReadMessage()
		if err != nil {
			gs.close()
			return
		}
		line := strings.TrimRight(string(msg), "\r\n")
		select {
		case gs.lines <- line:
		case <-gs.closed:
			return
		}
	}
}

func (gs *gameSession) writer() {
	for {
		select {
		case msg := <-gs.out:
			gs.conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
			if err := gs.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				gs.close()
				return
			}
		case <-gs.closed:
			// drain whatever made it into the buffer before hanging up
			for {
				select {
				case msg := <-gs.out:
					gs.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
					gs.conn.WriteMessage(websocket.TextMessage, msg)
				default:
					gs.conn.Close()
					return
				}
			}
		}
	}
}

func (gs *gameSession) run() {
	go gs.reader()
	go gs.writer()
	defer gs.close()

	u := gs.server.cfg.Universe

	var player *galwar.Player
	u.Do(func() {
		player = gs.server.lookupPlayer(gs.sub, gs.email)
	})

	gs.Printf("\r\n%s=== %sGALACTIC WARZONE%s ===%s\r\n", consoleui.Cyan, consoleui.White, consoleui.Cyan, consoleui.Reset)
	if player == nil {
		player = consoleui.RegisterFlow(u, gs, gs.email, gs.sub)
		if player == nil {
			return
		}
	} else {
		gs.Printf("\n%sWelcome back, %s!%s\n\n", consoleui.LightGreen, player.GetName(), consoleui.Reset)
	}

	gs.server.attach(player.Id, gs)
	defer gs.server.detach(player.Id, gs)

	u.Do(func() {
		u.TouchLastSeen(player, time.Now().Unix())
	})

	// reconstruction-after-death and news delivery; a player who died today
	// is turned away until tomorrow
	if !consoleui.SessionStart(u, gs, player) {
		time.Sleep(100 * time.Millisecond) // let the writer flush
		return
	}

	ui := consoleui.NewConsoleUI(u, player, gs)
	ui.Run()
	gs.Printf("\nGoodbye, %s!\n", player.GetName())
	// give the writer a moment to flush the farewell
	time.Sleep(100 * time.Millisecond)
}

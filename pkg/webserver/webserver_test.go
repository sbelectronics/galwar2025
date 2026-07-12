package webserver

import (
	"context"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/sbelectronics/galwar/pkg/consoleui"
	"github.com/sbelectronics/galwar/pkg/galwar"
)

func testServer(t *testing.T) (*Server, *galwar.UniverseType, string, func()) {
	t.Helper()
	u := galwar.NewUniverse()
	u.Generate(100)
	u.SeedDefaultConfig()
	u.Start()

	store, err := galwar.OpenStore(filepath.Join(t.TempDir(), "galwar.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	srv, err := New(context.Background(), Config{
		Universe:    u,
		Store:       store,
		DevAuth:     true,
		IdleTimeout: 30 * time.Second,
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	ts := httptest.NewServer(srv.Handler())
	cleanup := func() {
		srv.CloseAllSessions()
		ts.Close()
		store.Close()
	}
	return srv, u, ts.URL, cleanup
}

// wsClient wraps a logged-in websocket connection with read-until helpers.
type wsClient struct {
	t        *testing.T
	conn     *websocket.Conn
	buf      strings.Builder
	controls strings.Builder // NUL-prefixed control frames, concatenated
}

func dialGame(t *testing.T, base string, user string) *wsClient {
	t.Helper()

	// dev-auth to get the session cookie
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	resp, err := client.Get(base + "/auth/dev?user=" + url.QueryEscape(user))
	if err != nil {
		t.Fatalf("dev auth: %v", err)
	}
	resp.Body.Close()

	baseURL, _ := url.Parse(base)
	var cookieHeader []string
	for _, c := range jar.Cookies(baseURL) {
		cookieHeader = append(cookieHeader, c.Name+"="+c.Value)
	}
	if len(cookieHeader) == 0 {
		t.Fatalf("no session cookie after dev auth")
	}

	wsURL := "ws" + strings.TrimPrefix(base, "http") + "/ws"
	header := http.Header{
		"Cookie": {strings.Join(cookieHeader, "; ")},
		"Origin": {base},
	}
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("ws dial: %v", err)
	}
	return &wsClient{t: t, conn: conn}
}

// expect reads frames until the accumulated output contains substr.
func (c *wsClient) expect(substr string) string {
	c.t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		if strings.Contains(c.buf.String(), substr) {
			out := c.buf.String()
			c.buf.Reset()
			return out
		}
		c.conn.SetReadDeadline(deadline)
		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			c.t.Fatalf("waiting for %q: %v\ngot so far:\n%s", substr, err, c.buf.String())
		}
		if len(msg) > 0 && msg[0] == 0 {
			// out-of-band control frame, not terminal output
			c.controls.WriteString(string(msg[1:]))
			continue
		}
		// match on what a terminal would display, not the color codes
		c.buf.WriteString(consoleui.StripANSI(string(msg)))
	}
}

func (c *wsClient) send(line string) {
	c.t.Helper()
	if err := c.conn.WriteMessage(websocket.TextMessage, []byte(line)); err != nil {
		c.t.Fatalf("send %q: %v", line, err)
	}
}

func TestWebSysopBanFlow(t *testing.T) {
	_, u, base, cleanup := testServer(t)
	defer cleanup()

	// designate the admin (web-only: admins are keyed on email)
	u.Do(func() { u.SetConfig("admins", "boss@example.com") })

	// a victim registers, then leaves
	v := dialGame(t, base, "victim@example.com")
	v.expect("Choose your handle:")
	v.send("Victim Vic")
	v.expect("Welcome aboard")
	v.expect("Main Command")
	v.send("q")
	v.conn.Close()

	// the admin logs in and bans the victim through the sysop menu
	boss := dialGame(t, base, "boss@example.com")
	boss.expect("Choose your handle:")
	boss.send("Boss Hogg")
	boss.expect("Main Command")
	boss.send("sysop")
	boss.expect("Sysop (?=Help)")
	boss.send("b")
	boss.expect("Handle to ban?")
	boss.send("Victim Vic")
	boss.expect("Done")
	boss.send("q") // leave sysop menu
	boss.send("q") // quit
	boss.conn.Close()

	// the victim is banned in the engine, and the ban was audited
	var banned, audited bool
	u.Do(func() {
		if p := u.Players.GetByEmail("victim@example.com"); p != nil {
			banned = p.Banned
		}
		for _, a := range u.Audit {
			if a.Action == "ban" {
				audited = true
			}
		}
	})
	if !banned {
		t.Errorf("victim not banned after sysop ban")
	}
	if !audited {
		t.Errorf("ban not recorded in the audit log")
	}

	// the banned victim can no longer get into the game
	v2 := dialGame(t, base, "victim@example.com")
	v2.expect("suspended")
	v2.conn.Close()
}

func TestWebEndToEnd(t *testing.T) {
	_, u, base, cleanup := testServer(t)
	defer cleanup()

	// unauthenticated: /auth/me refuses, /ws refuses
	resp, err := http.Get(base + "/auth/me")
	if err != nil {
		t.Fatalf("me: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("unauthenticated /auth/me = %d; want 401", resp.StatusCode)
	}

	// new player: register through the terminal
	c := dialGame(t, base, "web@example.com")
	c.expect("Choose your handle:")
	c.send("fuck") // moderation in the loop
	c.expect("isn't allowed")
	c.send("Web Tester")
	c.expect("Welcome aboard, Web Tester!")
	c.expect("Main Command")
	c.send("i")
	c.expect("Credits: 35000")
	c.expect("Turns: 250")
	// PASS must put the browser in secret mode so the password is not echoed
	c.send("pass")
	c.expect("New password:")
	c.send("hunter2go")
	c.expect("Repeat password:")
	c.send("hunter2go")
	c.expect("Password set")
	if got := c.controls.String(); !strings.Contains(got, "secret:on") || !strings.Contains(got, "secret:off") {
		t.Errorf("PASS did not toggle web secret mode; control frames = %q", got)
	}

	c.send("q")
	c.expect("Goodbye, Web Tester!")
	c.conn.Close()

	// the telnet password we just set works
	u.Do(func() {
		if p := u.Players.GetByEmail("web@example.com"); p == nil || !p.CheckTelnetPassword("hunter2go") {
			t.Errorf("PASS did not set a working telnet password")
		}
	})

	// the player exists with the dev sub
	var sub string
	u.Do(func() {
		if p := u.Players.GetByEmail("web@example.com"); p != nil {
			sub = p.GoogleSub
		}
	})
	if sub != "dev:web@example.com" {
		t.Fatalf("player sub = %q; want dev:web@example.com", sub)
	}

	// returning player: no registration prompt
	c2 := dialGame(t, base, "web@example.com")
	c2.expect("Welcome back, Web Tester!")
	c2.expect("Main Command")

	// session takeover: second connection boots the first
	c3 := dialGame(t, base, "web@example.com")
	c3.expect("Welcome back, Web Tester!")
	c2.expect("Another session has taken over")
	c3.send("q")
	c3.expect("Goodbye")
	c2.conn.Close()
	c3.conn.Close()
}

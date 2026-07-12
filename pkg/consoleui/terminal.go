package consoleui

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/sbelectronics/galwar/pkg/galwar"
)

// Terminal is the game UI's only view of the outside world: something that
// shows text and yields lines. The console, a WebSocket session, and a
// telnet connection all implement it, which is what lets one UI serve three
// front-ends.
type Terminal interface {
	Printf(format string, args ...any)
	// ReadLine blocks for the next line of input. An error (EOF, disconnect,
	// idle timeout) ends the session.
	ReadLine() (string, error)
}

// SecretReader is optionally implemented by terminals that can suppress
// echo for password entry. Callers fall back to ReadLine when absent.
type SecretReader interface {
	ReadSecret() (string, error)
}

// StdioTerminal drives the local console. Color is enabled when stdout is a
// VT-capable console; piped output is automatically colorless.
type StdioTerminal struct {
	scanner *bufio.Scanner
	Color   bool
}

func NewStdioTerminal() *StdioTerminal {
	return &StdioTerminal{
		scanner: bufio.NewScanner(os.Stdin),
		Color:   enableVT(),
	}
}

func (t *StdioTerminal) Printf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if !t.Color {
		msg = StripANSI(msg)
	}
	fmt.Print(msg)
}

func (t *StdioTerminal) ReadLine() (string, error) {
	if !t.scanner.Scan() {
		if err := t.scanner.Err(); err != nil {
			return "", err
		}
		return "", io.EOF
	}
	return t.scanner.Text(), nil
}

// ReadPassword reads a secret, using echo suppression when the terminal
// supports it.
func ReadPassword(term Terminal) (string, error) {
	if sr, ok := term.(SecretReader); ok {
		return sr.ReadSecret()
	}
	return term.ReadLine()
}

// ErrText renders an error the way the game shows it to players: the bare
// message for game-rule errors, prefixed for unexpected ones.
func ErrText(err error) string {
	if gameErr, ok := err.(*galwar.GameError); ok {
		return gameErr.Message()
	}
	return "Error: " + err.Error()
}

// SessionStart runs the login-time rituals shared by every front-end:
// reconstruction after death (with the day-of-death lockout) and delivery
// of the player's news. Returns false when the session must end (the
// player died today and the Traders Guild hasn't finished with them).
func SessionStart(u *galwar.UniverseType, term Terminal, player *galwar.Player) bool {
	var deathMsg string
	var alive bool
	var news []string
	u.Do(func() {
		deathMsg, _ = u.ReconstructIfDue(player, time.Now())
		alive = !player.IsDead()
		if alive {
			news = u.TakeNews(player.Id)
		}
	})

	if deathMsg != "" {
		term.Printf("\n%s%s%s\n", LightRed, deathMsg, Reset)
	}
	if !alive {
		return false
	}
	PrintNews(term, "While you were away:", news)
	return true
}

// PrintNews renders a batch of news items under a header, in the original's
// news colors (yellow header, light-cyan items). A no-op when empty.
func PrintNews(term Terminal, header string, news []string) {
	if len(news) == 0 {
		return
	}
	term.Printf("\n%s%s%s\n", Yellow, header, Reset)
	for _, msg := range news {
		term.Printf("%s  %s%s\n", LightCyan, msg, Reset)
	}
}

// RegisterFlow prompts for a handle until one passes moderation and
// uniqueness, then creates the player. Shared by the web and telnet
// front-ends. Returns nil if the session ends or the player gives up.
func RegisterFlow(u *galwar.UniverseType, term Terminal, email string, googleSub string) *galwar.Player {
	term.Printf("\n%sWelcome, new trader!%s\n", LightGreen, Reset)
	term.Printf("Your handle is how other players will know you.\n")
	for attempt := 0; attempt < 8; attempt++ {
		term.Printf("\nChoose your handle: ")
		line, err := term.ReadLine()
		if err != nil {
			return nil
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var player *galwar.Player
		rerr := u.DoErr(func() error {
			p, err := u.RegisterPlayer(line, email, googleSub)
			player = p
			return err
		})
		if rerr != nil {
			term.Printf("%s\n", ErrText(rerr))
			continue
		}
		term.Printf("\nWelcome aboard, %s!\n", player.GetName())
		return player
	}
	term.Printf("Too many attempts. Reconnect to try again.\n")
	return nil
}

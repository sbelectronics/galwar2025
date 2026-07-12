// Thin terminal client: local line editing + echo, full lines to the server.
// The server is the game; this file is deliberately dumb.

(async function () {
  const login = document.getElementById("login");
  const termDiv = document.getElementById("terminal");

  let me;
  try {
    const resp = await fetch("/auth/me");
    if (!resp.ok) throw new Error("unauthenticated");
    me = await resp.json();
  } catch (e) {
    login.style.display = "block";
    return;
  }
  if (me.dev) document.getElementById("devhint").style.display = "block";

  termDiv.style.display = "block";
  const term = new Terminal({
    cursorBlink: true,
    fontFamily: '"Consolas", "Courier New", monospace',
    fontSize: 16,
    theme: { background: "#000000", foreground: "#c0c0c0", cursor: "#55ff55" },
  });
  const fit = new FitAddon.FitAddon();
  term.loadAddon(fit);
  term.open(termDiv);
  fit.fit();
  window.addEventListener("resize", () => fit.fit());

  const proto = location.protocol === "https:" ? "wss" : "ws";
  const ws = new WebSocket(`${proto}://${location.host}/ws`);

  ws.onmessage = (ev) => term.write(ev.data);
  ws.onclose = () => term.write("\r\n\r\n*** Connection closed. Reload the page to reconnect. ***\r\n");
  ws.onerror = () => term.write("\r\n*** Connection error ***\r\n");

  let buf = "";
  term.onData((d) => {
    if (ws.readyState !== WebSocket.OPEN) return;
    for (const ch of d) {
      if (ch === "\r") {
        term.write("\r\n");
        ws.send(buf);
        buf = "";
      } else if (ch === "\x7f" || ch === "\b") {
        if (buf.length > 0) {
          buf = buf.slice(0, -1);
          term.write("\b \b");
        }
      } else if (ch >= " ") {
        buf += ch;
        term.write(ch);
      }
    }
  });

  term.focus();
})();

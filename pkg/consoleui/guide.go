package consoleui

import (
	"fmt"
	"strings"
)

// The player guide - the Z command. Written in the voice of the original
// GWINSTR documentation: second person, a little breathless, typed-at-2-AM
// charm. The prose is allowed to be rough; the FACTS are not - every command,
// price, and rule below matches the engine (guide_test.go proofreads it, and
// checks that nothing unimplemented is documented as if it worked).

type guideSection struct {
	title string
	body  string
}

var guideSections = []guideSection{
	{"Background: The War", `
For several centuries the Federation has been at war with a race of ruthless
aliens known as The Cabal. The Cabal have a nasty habit of roaming the galaxy
and destroying whatever they run into, for no particular reason. For a long
while they were merely in the way. They are not merely in the way anymore.

The Federation is no saint either. It has grown corrupt, and stopped enforcing
galactic law out past the core worlds. Pirates - the Renegades - now raid where
they please.

So where do you fit in? You just happen to be one of these cunning and
unrestrained traders, and your sole ambition in life is, of course, universal
dominance. You get there by growing richer and deadlier than every rival on the
board - by climbing the Greatest Warlords rankings and staying there while
everyone else tries to knock you off. It is, by no means, an easy task!`},

	{"Getting Started", `
You begin at Sol (sector 1) with a starter ship: 35,000 credits, 250 turns for
the day, 200 fighters, and 25 cargo holds. Sectors 1 through 10 are Federation
space - no one may attack you, drop mines, or build there. It is a safe place
to learn the ropes. It is NOT a place to get rich.

HERE IS THE MOST IMPORTANT THING IN THIS ENTIRE MANUAL: your 35,000 credits are
not spending money. They are a down payment on cargo holds. Dock at Sol (the P
command), buy as many cargo holds as you can afford at 500 credits each, and
THEN go trading. A ship with 25 holds earns pocket change; a ship with a couple
hundred holds earns a living. Traders who skip this step decide the game is
boring and wander off. Don't be that trader.

Then find a port that sells a good cheaply and another that buys it dearly, and
run cargo between them. Use the computer's "find the nearest port" (C, then F)
so you aren't flying blind across two thousand sectors.`},

	{"Main Menu Commands", `
At the Main Command prompt, a single letter does the following. (A prompt
asking for a number will take Q to back out - so a fat-finger never traps you.
Careful with bare Enter, though: if the prompt shows a default, Enter TAKES the
default; only a prompt with no default treats Enter as backing out.)

<A> - ATTACK. Attack another trader in your sector. You choose how many
      fighters to commit. If you don't wipe them out at once, they may
      counter-attack - possibly spreading your atoms across the universe!

<M> - MOVE. Jump to a warp-linked sector. Costs one turn. Your first move
      makes your ship visible to the rest of the galaxy (a ship that never
      leaves dock is nobody's business).

<S> - SENSOR SCAN. Look into every sector adjacent to yours.

<Y> - AUTOPILOT. Plot the shortest course to any sector and load it in, so you
      just tap Enter to fly the route.

<P> - DOCK AT PORT. Trade at the port in your sector. Sol, Amazing Devices, and
      the Interstel bank are all reached this way.

<D> / <F> - Drop MINES / leave FIGHTERS as a sector defense force. Not allowed
      in sectors 1 through 10. Anyone entering the sector fights your fighters
      and trips your mines.

<G> - LAUNCH GROUP. Send a battle group of fighters to a distant sector; they
      fight through everything on the way and the survivors come home. Send
      ONE ship and it's a scout - it recons but won't pick fights.

<J> - CREATE PLANET. Use a Genesis Device to found a planet (not in sectors
      1-10). See the Planets section.

<L> - LAND / INVADE. Land on your own planet to manage it, or assault someone
      else's.

<B> - USE DEVICE. Fire an activatable device you own (the Pulsar Tube).

<H> - DAMAGE CONTROL. List your damaged ship systems. Damage heals one point
      per turn you spend, or all at once for a price at Sol.

<I> - INFO. Your name, credits, cargo, fighters, devices, bank balance.

<C> - COMPUTER. Read-only reports. See the next section.

<W> - PLASMA DEVICE. Rewire the warp map (see Devices).

<Z> - INSTRUCTIONS. This manual.

<Q> - QUIT. Log off. Your game is saved.

A few hidden commands, typed in full: PASS sets a telnet password; REPORT flags
a misbehaving trader for the sysop.`},

	{"The Computer", `
Your ship's computer (the C command) offers reports, provided your computer
isn't shot up. Type the letter, or ? for the list:

  [L] Rank the greatest warlords - the leaderboard, richest first.
  [E] Find your forces - every planet and sector defense force you own, and
      where you left it. Invaluable once you're spread out.
  [P] Your planetary status - production and stockpiles on your planets,
      without flying to each one.
  [F] Find the nearest port - the closest port, or the closest one buying or
      selling a good you name, with a course to it.
  [U] Universe specifics - sectors, ports, planets, and how many traders are
      active right now. (If that last number is healthy, you've got company.)
  [W] What happened - re-show recent news you may have scrolled past.`},

	{"Ports and Trading", `
Ordinary ports trade Ore, Organics, and Equipment. Each port buys some goods
and sells others; the price drifts with how full the port's shelves are, so a
picked-over port charges a little more for what you take AND pays a little less
for what you bring. Fresh shelves mean fair prices - spread your business
around. Buy low, sell high, repeat.

Bigger ships trade in bulk: past 50 holds your quantities scale up, so a proper
freighter fills its holds in one visit instead of a hundred.

Three special ports live in Federation space, all free to dock and turn-less:

  SOL (sector 1) - Federation Operations. Buy cargo holds (500), fighters (98),
    turns (1,500 each), Genesis Devices (10,000), mines (15,000), and the
    stockpile weapons: Plasma (56,000), Pulsar bombs (215,000), Emergency Warp
    (27,000). Sol also does full ship repairs, per point of damage.

  AMAZING DEVICES - the device shop. See the Devices section.

  INTERSTEL - the bank. Deposit and withdraw credits. Your account earns a
    little interest each night (on the first million), and - this is the point
    - it SURVIVES THE DESTRUCTION OF YOUR SHIP. Credits in your holds are lost
    when you die; credits in the bank are not. Decide how much to keep liquid.`},

	{"Planets", `
A Genesis Device (10,000 at Sol) founds a planet in your current sector, if
you're outside Federation space and no planet is there already. Name it and
it's yours.

Planets produce Ore, Organics, and Equipment on their own each night, and the
production compounds as stockpiles grow - a mature planet is a fortune that
mines itself. Land on your planet (the L command) to run it: take cargo off,
transfer cargo and fighters on, and garrison it with fighters against invaders.
A planet grows its own minefield too, given time - production turns out mines
along with everything else - so a mature world defends itself twice over.

Someone else's planet is a target. Land on it and you invade instead: your
fighters fight the garrison, then the planet's mines go off in your face. Break
through both and the planet is yours - though a hard-fought assault wrecks its
production, and a truly savage one can level the planet entirely.`},

	{"Combat and Death", `
When you Attack a trader, your committed fighters and theirs trade blows until
one side breaks. Survive, and if they still have fighters there's a good chance
they counter-attack with everything left. Win, and you salvage a share of their
cargo holds from the wreck.

WATCH THE MINES. If the trader you just killed was hauling sector mines, every
one of them detonates against YOU as their ship comes apart - and a big enough
stockpile will take you down with them. Think twice before finishing off a
mine-layer.

An Emergency Warp device saves you: at the instant your fighters hit zero, it
fires and flings you to a random sector instead of the graveyard.

If you do die, the Traders Guild reconstructs your ship the next day. Your FIRST
death is free. After that each reconstruction costs you an escalating penalty,
so dying becomes a habit you can't afford. Your bank balance, remember, comes
through death untouched.`},

	{"Devices", `
The Amazing Devices port sells gear beyond Sol's stockpile weapons. Owning one
of a device is enough - a second copy does nothing.

  Cloaking Device (18,000) - hides your ship from other traders.
  Anti-Cloaking Device (22,000) - lets you see (and attack) cloaked ships.
  Pulsar Tube (350,000) - launch pulsar bombs at any planet in your sector
    from orbit, owned by you or not. The [B] Use Device command.
  Fusion Cell (45,000) - banks your unused turns overnight (up to a full day's
    worth) and spends them automatically once tomorrow's turns run out. If you
    can only play now and then, buy this first.
  Planetary Scanner (40,000) - reveals a hostile planet's fighters and mines
    before you commit to invading it. Turns a blind gamble into a decision.
  Mine Deflector (6,000 each) - a stack of one-shot charges; each absorbs one
    mine blast that would have hit your ship. Buy a handful before a minefield.

From Sol's stockpile: the Plasma Device (the W command) rewires warps - adding
or cutting a two-way link between your sector and another. It won't touch
sectors 1 through 10. Pulsar bombs wreck planetary production. Genesis founds
planets. Emergency Warp saves your life, once each.`},

	{"The Powers That Be", `
Three factions share the galaxy with you, and they wake and sleep according to
how lively the game is:

  THE FEDERATION keeps Federation space (sectors 1-10) safe, always. When the
  other factions stir, its fleets take the field to fight them; when the galaxy
  is quiet, the fleet stands down.

  THE CABAL is the scaling menace. It sleeps while the galaxy is full of
  beginners - it has no interest in crushing newbies. But let a warlord grow
  mighty enough to be worth crushing, and the Cabal gates in a fortress scaled
  to that leader and comes hunting them. Climb to the top and you inherit its
  full attention.

  THE RENEGADES are ambient chaos - pirates who raid non-Federation space once
  there are real players about to make the danger interesting. Like the Cabal,
  they leave the small and the new alone.

The moral: nobody bullies a beginner here. The factions only exist for, and
only strike at, players who have genuinely earned the trouble.`},

	{"Practical Matters", `
TURNS. You get a fresh allowance (250) every day; unused turns do NOT carry
over from one day to the next - unless you own a Fusion Cell, which banks them
for you. Spending a turn also heals one point of ship-system damage.

DORMANCY. Stop logging in for several days and your ship goes quiet - hidden
from other players, safe from attack, but its planets stop growing. Log in and
you're instantly back. Take a truly long vacation, though - a month - and the
Traders Guild repossesses your holdings and hands you a fresh starter ship.
Everything goes - planets, garrisons, and yes, the bank account (the bank
forgives death, not desertion). Your handle survives; the empire does not.

MISBEHAVIOR. If another trader is harassing or offending, REPORT them (type it
in full) and a sysop will review it. Handles and planet names are screened, so
keep it clean.

That's the game. Buy holds, get rich, build planets, watch the mines, and try
to still be standing at the top of the rankings when the Cabal comes calling.
Good luck, trader. You'll need it.`},
}

// ExecuteGuide is the Z command: a menu of guide sections, each paged.
func (c *ConsoleUI) ExecuteGuide() {
	for !c.Terminated {
		c.printf("\n%s          GALACTIC WARZONE - Trader's Manual%s\n", Yellow, Reset)
		c.printf("%s~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~%s\n", Green, Reset)
		for i, s := range guideSections {
			c.printf("  %s[%d]%s %s\n", Cyan, i+1, Reset, s.title)
		}
		choice, ok := c.PromptInt("\nRead which section (number, or Q to quit) ? ")
		if !ok || choice < 1 || choice > len(guideSections) {
			return
		}
		c.pageText(guideSections[choice-1].title, guideSections[choice-1].body)
	}
}

// guidePageSize is the number of body lines shown per screen by the pager.
const guidePageSize = 20

// pageText prints body a screen at a time, waiting for Enter between screens
// and letting Q stop early. Generic: any long text can use it.
func (c *ConsoleUI) pageText(title, body string) {
	c.printf("\n%s%s%s\n", Yellow, title, Reset)
	c.printf("%s~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~%s\n", Green, Reset)

	lines := strings.Split(strings.TrimRight(body, "\n"), "\n")
	for i := 0; i < len(lines); {
		end := i + guidePageSize
		if end > len(lines) {
			end = len(lines)
		}
		for ; i < end; i++ {
			c.printf("%s\n", lines[i])
		}
		if i >= len(lines) {
			break
		}
		resp := strings.ToLower(strings.TrimSpace(c.PromptString(fmt.Sprintf("%s-- more -- <Enter> to continue, Q to stop --%s ", Cyan, Reset))))
		if c.Terminated || resp == "q" {
			break
		}
	}
}

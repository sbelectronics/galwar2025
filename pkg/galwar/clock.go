package galwar

import "time"

// Now is the engine's single source of wall-clock time. Production leaves it
// as time.Now; the simulation harness (pkg/botsim) overrides it with a
// synthetic clock so it can compress days into minutes - dormancy, expiry,
// interest, restock cooldowns, and the faction quiet-day logic all key off
// this, so advancing it +24h makes the world experience a full day.
//
// It is a package-level var, deliberately: there is exactly one universe clock
// per process. Overriding it is a test/simulation affordance, not a per-call
// parameter - the engine reads it in too many leaf functions (audit stamps,
// combat timestamps, port restock) to thread a clock through every signature.
// Nothing in production writes to it.
var Now = time.Now

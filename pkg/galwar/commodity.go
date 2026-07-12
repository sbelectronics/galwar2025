package galwar

type Commodity struct {
	Name        string
	Prod        int
	Quantity    int
	BuyPrice    float64
	SellPrice   float64
	Sell        bool
	LastRestock int64 // unix seconds; 0 = not yet initialized
}

type CommodityDefinition struct {
	Commodity
	ShortName          string
	Holds              int
	Starting           int
	SellAtPorts        bool
	SellAtSol          bool
	OnPlanet           bool
	PlanetStarting     int
	PlanetProdStarting int
}

const (
	ORE       = "Ore"
	ORGANICS  = "Organics"
	EQUIPMENT = "Equipment"
	HOLDS     = "Cargo Holds"
	FIGHTERS  = "Fighters"
	MINES     = "Mines"
	GENESIS   = "Genesis Devices"
	TURNS     = "Turns"
	PLASMA    = "Plasma Devices"
	PULSAR    = "Pulsar Bombs"
	EMWARP    = "Emergency Warp"
)

var TradeGoods = []CommodityDefinition{
	{Commodity: Commodity{Name: ORE, BuyPrice: 8, SellPrice: 5}, ShortName: "Ore", Holds: 1, SellAtPorts: true, OnPlanet: true, PlanetStarting: 10, PlanetProdStarting: 1},
	{Commodity: Commodity{Name: ORGANICS, BuyPrice: 14, SellPrice: 10}, ShortName: "Org", Holds: 1, SellAtPorts: true, OnPlanet: true, PlanetStarting: 10, PlanetProdStarting: 1},
	{Commodity: Commodity{Name: EQUIPMENT, BuyPrice: 25, SellPrice: 20}, ShortName: "Equ", Holds: 1, SellAtPorts: true, OnPlanet: true, PlanetStarting: 10, PlanetProdStarting: 1},
	{Commodity: Commodity{Name: HOLDS, SellPrice: 500, Sell: true}, ShortName: "Holds", Holds: 0, Starting: 25, SellAtSol: true},
	{Commodity: Commodity{Name: FIGHTERS, SellPrice: 98, Sell: true}, ShortName: "Fighters", Holds: 0, Starting: 200, SellAtSol: true, OnPlanet: true},
	{Commodity: Commodity{Name: MINES, SellPrice: 15000, Sell: true}, ShortName: "Mines", Holds: 0, SellAtSol: true, OnPlanet: true},
	{Commodity: Commodity{Name: GENESIS, SellPrice: 10000, Sell: true}, ShortName: "Genesis", Holds: 0, SellAtSol: true},
	{Commodity: Commodity{Name: PLASMA, SellPrice: 56000, Sell: true}, ShortName: "Plasma", Holds: 0, SellAtSol: true},
	{Commodity: Commodity{Name: PULSAR, SellPrice: 215000, Sell: true}, ShortName: "Pulsar", Holds: 0, SellAtSol: true},
	{Commodity: Commodity{Name: EMWARP, SellPrice: 27000, Sell: true}, ShortName: "EmWarp", Holds: 0, SellAtSol: true},
	{Commodity: Commodity{Name: TURNS, SellPrice: 1500, Sell: true}, ShortName: "Turns", Holds: 0, Starting: 250, SellAtSol: true},
}

// FindCommodityDef returns the definition for a commodity name, or nil if
// there is none. Universe.Load validates every stored commodity against this,
// so GetDef's panic below can only fire on a programming error, not on data.
func FindCommodityDef(name string) *CommodityDefinition {
	for i := range TradeGoods {
		if name == TradeGoods[i].Name {
			return &TradeGoods[i]
		}
	}
	return nil
}

func (c *Commodity) GetDef() *CommodityDefinition {
	def := FindCommodityDef(c.Name)
	if def == nil {
		panic("Fatal error: Commodity definition not found: " + c.Name)
	}
	return def
}

func (c *Commodity) GetShortName() string {
	return c.GetDef().ShortName
}

func (c *Commodity) GetHoldsUsed() int {
	return c.GetDef().Holds * c.Quantity
}

func (c *Commodity) IsCargo() bool {
	return c.GetDef().Holds > 0
}

// DepletionFraction is how empty this commodity's stock is relative to its
// maximum (Prod*10), from 0.0 (full) to 1.0 (empty). Commodities without a
// production rate (ship equipment, Sol goods) never deplete.
func (c *Commodity) DepletionFraction() float64 {
	max := c.Prod * 10
	if max <= 0 {
		return 0
	}
	d := float64(max-c.Quantity) / float64(max)
	if d < 0 {
		return 0
	}
	if d > 1 {
		return 1
	}
	return d
}

// Effective prices swing up to 5% with scarcity, per the original's GetPrice
// (GENPORT.PAS): the emptier a port's stock, the more it charges for what it
// sells and the less it pays for what it buys.

func (c *Commodity) EffectiveBuyPrice() float64 {
	return c.BuyPrice * (1 - 0.05*c.DepletionFraction())
}

func (c *Commodity) EffectiveSellPrice() float64 {
	return c.SellPrice * (1 + 0.05*c.DepletionFraction())
}

func (c *Commodity) GetBuyPrice(quantity int) int {
	return int(c.EffectiveBuyPrice() * float64(quantity))
}

func (c *Commodity) GetSellPrice(quantity int) int {
	return int(c.EffectiveSellPrice() * float64(quantity))
}

func (c *Commodity) GetPrice() float64 {
	if c.Sell {
		return c.EffectiveSellPrice()
	}
	return c.EffectiveBuyPrice()
}

// Restock lazily accrues production since the last accrual: +2*Prod per day,
// capped at 10*Prod, matching the original nightly update_ports
// (MAINT1.PAS:251-269) integrated continuously. The clock advances only by
// the seconds actually consumed by whole units, so rapid repeated calls can
// never round the accrual away.
func (c *Commodity) Restock(now int64) {
	if c.Prod <= 0 {
		return
	}
	if c.LastRestock == 0 {
		c.LastRestock = now
		return
	}
	elapsed := now - c.LastRestock
	if elapsed <= 0 {
		return
	}
	max := c.Prod * 10
	if c.Quantity >= max {
		c.LastRestock = now
		return
	}
	rate := int64(2 * c.Prod) // units per day
	gain := rate * elapsed / 86400
	if gain <= 0 {
		return
	}
	c.LastRestock += gain * 86400 / rate
	c.Quantity += int(gain)
	if c.Quantity >= max {
		c.Quantity = max
		c.LastRestock = now
	}
}

func (c Commodity) GetBuySell() string {
	if c.Sell {
		return "Selling"
	} else {
		return "Buying"
	}
}

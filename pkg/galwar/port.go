package galwar

import (
	"fmt"
	"strings"
)

type PortType int

const (
	TradingPort    PortType = iota // regular ore/org/eqp trading port
	Sol                            // Federation Operations: ship gear, turns, plasma/pulsar/emwarp
	AmazingDevices                 // the device shop: cloak, anti-cloak, pulsar tube
)

type Port struct {
	Goods PortType
	ObjectBase
	InventoryBase
}

type PortList struct {
	Ports []*Port
}

// IsService reports whether this is a special-function port rather than a
// regular commodity-trading port. It is deliberately "any non-trading port":
// Sol and Amazing Devices today, and any future special port (the Interstel
// bank, Casino, Vault, Sanctuary) - all of which are free, turn-less, and
// fixed-inventory by design. Only trading ports charge a turn and restock, so
// new special ports get service treatment automatically and needn't be listed
// here. Add a new PortType to the trading side only if it should charge turns.
func (p *Port) IsService() bool {
	return p.Goods != TradingPort
}

func (p *Port) GetNameExtra() string {
	switch p.Goods {
	case Sol:
		return "Federation Operations"
	case AmazingDevices:
		return "Amazing Devices"
	}

	sellNames := []string{}
	for _, c := range p.Inventory {
		if c.Sell {
			sellNames = append(sellNames, c.GetShortName())
		}
	}
	if len(sellNames) == 0 {
		return "Selling: None"
	} else {
		return fmt.Sprintf("Selling: %s", strings.Join(sellNames, ", "))
	}
}

func (p *Port) GetType() string {
	return TYPE_PORT
}

// Restock lazily accrues production for every commodity the port trades.
// Sol goods have no production rate, so this is naturally a no-op there.
func (p *Port) Restock(now int64) {
	for _, c := range p.Inventory {
		c.Restock(now)
	}
}

func (p *PortList) GetObjectsInSector(sector int) []ObjectInterface {
	var portsInSector []ObjectInterface
	for _, port := range p.Ports {
		if port.Sector == sector {
			portsInSector = append(portsInSector, port)
		}
	}
	return portsInSector
}

// ensureAmazingDevices makes sure the universe has an Amazing Devices port
// stocking every SellAtDevices commodity. Idempotent: creates the port at a
// free low sector if absent, and tops up an existing one with any devices it
// is missing (so a universe from before a new device was added learns to
// sell it). Safe to call from Generate and from upgrade.
func (u *UniverseType) ensureAmazingDevices() {
	var port *Port
	for _, p := range u.Ports.Ports {
		if p.Goods == AmazingDevices {
			port = p
			break
		}
	}
	if port == nil {
		sector := u.freePortSector()
		if sector == 0 {
			return // universe is wall-to-wall ports; nothing we can do
		}
		port = &Port{
			Goods:      AmazingDevices,
			ObjectBase: ObjectBase{Name: "Amazing Devices, Inc.", Sector: sector},
		}
		u.Ports.Ports = append(u.Ports.Ports, port)
	}
	for _, tg := range TradeGoods {
		if !tg.SellAtDevices || port.GetCommodity(tg.Name) != nil {
			continue
		}
		cm := tg.Commodity
		cm.Sell = true
		port.Inventory = append(port.Inventory, &cm)
	}
}

// freePortSector returns a portless sector, preferring the low (Federation)
// sectors where the original kept its service ports; 0 if none is free.
func (u *UniverseType) freePortSector() int {
	for s := 2; s <= 10 && s < len(u.Sectors); s++ {
		if len(u.GetObjectsInSector(s, TYPE_PORT)) == 0 {
			return s
		}
	}
	for s := 2; s < len(u.Sectors); s++ {
		if len(u.GetObjectsInSector(s, TYPE_PORT)) == 0 {
			return s
		}
	}
	return 0
}

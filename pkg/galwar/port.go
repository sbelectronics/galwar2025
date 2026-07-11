package galwar

import (
	"fmt"
	"strings"
)

type PortType int

const (
	TradingPort PortType = iota
	Sol
)

type Port struct {
	Goods PortType
	ObjectBase
	InventoryBase
}

type PortList struct {
	Ports []*Port
}

func (p *Port) GetNameExtra() string {
	if p.Goods == Sol {
		return "Federation Operations"
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

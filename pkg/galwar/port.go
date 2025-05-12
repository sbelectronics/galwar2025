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
	Name      string
	Sector    int
	Inventory []Commodity
	Type      PortType
}

type PortList struct {
	Ports []*Port
}

func (p *Port) GetName() string {
	return p.Name
}

func (p *Port) GetNameExtra() string {
	if p.Type == Sol {
		return "Federation Operations"
	}

	sellNames := []string{}
	for _, c := range p.Inventory {
		if c.Sell {
			sellNames = append(sellNames, c.ShortName)
		}
	}
	if len(sellNames) == 0 {
		return "Selling: None"
	} else {
		return fmt.Sprintf("Selling: %s", strings.Join(sellNames, ", "))
	}
}

func (p *Port) GetType() string {
	return "Port"
}

func (p *Port) GetSector() int {
	return p.Sector
}

func (p *Port) GetCommodities() []Commodity {
	return p.Inventory
}

func (p *Port) GetQuantity(name string) int {
	for _, c := range p.Inventory {
		if c.Name == name {
			return c.Quantity
		}
	}
	return 0
}

func (p *Port) GetCommodity(name string) *Commodity {
	for _, c := range p.Inventory {
		if c.Name == name {
			return &c
		}
	}
	return nil
}

func (p *Port) AdjustQuantity(name string, amount int) {
	for i, c := range p.Inventory {
		if c.Name == name {
			p.Inventory[i].Quantity += amount
			return
		}
	}
	//return fmt.Errorf("commodity %s not found in port %s", name, p.Name)
}

func (p *Port) GetMoney() {
	// Ports don't have money
}

func (p *Port) AdjustMoney(amount int) {
	// Ports don't have money, so this is a no-op
	_ = amount
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

var Ports = &PortList{}

func init() {
	Universe.Register(Ports)
}

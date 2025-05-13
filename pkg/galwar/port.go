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

func (p *PortList) GetObjectsInSector(sector int) []ObjectInterface {
	var portsInSector []ObjectInterface
	for _, port := range p.Ports {
		if port.Sector == sector {
			portsInSector = append(portsInSector, port)
		}
	}
	return portsInSector
}

var Ports = PortList{}

func init() {
	Universe.RegisterPorts(&Ports)
}

package ifmibpoller

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/soniah/gosnmp"
)

var (
	errorsIndexDrift = errors.New("Inconsistent index")
)

const (
	SNMPv2_TC_TruthValue_True  = 1
	SNMPv2_TC_TruthValue_False = 2

	OID_ifMIB_ifXEntry = ".1.3.6.1.2.1.31.1.1.1"

	// State
	OID_ifXEntry_ifName             = ".1.3.6.1.2.1.31.1.1.1.1"
	OID_ifXEntry_ifHighSpeed        = ".1.3.6.1.2.1.31.1.1.1.15"
	OID_ifXEntry_ifConnectorPresent = ".1.3.6.1.2.1.31.1.1.1.17"
	OID_ifXEntry_ifAlias            = ".1.3.6.1.2.1.31.1.1.1.18"

	// Measurement
	OID_ifXEntry_ifHCInOctets         = ".1.3.6.1.2.1.31.1.1.1.6"
	OID_ifXEntry_ifHCInUcastPkts      = ".1.3.6.1.2.1.31.1.1.1.7"
	OID_ifXEntry_ifHCInMulticastPkts  = ".1.3.6.1.2.1.31.1.1.1.8"
	OID_ifXEntry_ifHCInBroadcastPkts  = ".1.3.6.1.2.1.31.1.1.1.9"
	OID_ifXEntry_ifHCOutOctets        = ".1.3.6.1.2.1.31.1.1.1.10"
	OID_ifXEntry_ifHCOutUcastPkts     = ".1.3.6.1.2.1.31.1.1.1.11"
	OID_ifXEntry_ifHCOutMulticastPkts = ".1.3.6.1.2.1.31.1.1.1.12"
	OID_ifXEntry_ifHCOutBroadcastPkts = ".1.3.6.1.2.1.31.1.1.1.13"
)

type IfStats struct {
	Timestamp time.Time     `json:"-"`
	Duration  time.Duration `json:"-"`

	State IfMibState
	Count IfMibCount
}

type IfMibState struct {
	Index     []uint64
	Name      []string
	Alias     []string
	Present   []bool
	Linkspeed []uint64
}

type IfMibCount struct {
	InOctets         []uint64
	InUcastPkts      []uint64
	InMulticastPkts  []uint64
	InBroadcastPkts  []uint64
	OutOctets        []uint64
	OutUcastPkts     []uint64
	OutMulticastPkts []uint64
	OutBroadcastPkts []uint64
}

func (x *IfMibState) Equal(y *IfMibState) bool {
	if len(x.Index) != len(y.Index) ||
		len(x.Name) != len(y.Name) ||
		len(x.Alias) != len(y.Alias) ||
		len(x.Present) != len(y.Present) ||
		len(x.Linkspeed) != len(y.Linkspeed) {
		return false
	}

	for i, xi := range x.Index {
		if xi != y.Index[i] {
			return false
		}
	}

	for i, xi := range x.Name {
		if xi != y.Name[i] {
			return false
		}
	}

	for i, xi := range x.Alias {
		if xi != y.Alias[i] {
			return false
		}
	}

	for i, xi := range x.Present {
		if xi != y.Present[i] {
			return false
		}
	}

	for i, xi := range x.Linkspeed {
		if xi != y.Linkspeed[i] {
			return false
		}
	}

	return true
}

func (x *IfMibCount) Equal(y *IfMibCount) bool {
	ptrs := []*[]uint64{
		&x.InOctets, &y.InOctets,
		&x.InUcastPkts, &y.InUcastPkts,
		&x.InMulticastPkts, &y.InMulticastPkts,
		&x.InBroadcastPkts, &y.InBroadcastPkts,
		&x.OutOctets, &y.OutOctets,
		&x.OutUcastPkts, &y.OutUcastPkts,
		&x.OutMulticastPkts, &y.OutMulticastPkts,
		&x.OutBroadcastPkts, &y.OutBroadcastPkts,
	}

	for i := 0; i < len(ptrs); i += 2 {
		x := *ptrs[i]
		y := *ptrs[i+1]

		if len(x) != len(y) {
			return false
		}

		for j, xj := range x {
			if xj != y[j] {
				return false
			}
		}

	}

	return true
}

func (st *IfStats) Walk(snmp *gosnmp.GoSNMP) error {
	var err error

	st.Timestamp = time.Now()

	err = st.walkName(snmp)
	if err != nil {
		return err
	}

	st.State.Linkspeed, err = st.walkUint64(snmp, OID_ifXEntry_ifHighSpeed)
	if err != nil {
		return err
	}

	st.State.Present, err = st.walkBool(snmp, OID_ifXEntry_ifConnectorPresent)
	if err != nil {
		return err
	}

	st.State.Alias, err = st.walkString(snmp, OID_ifXEntry_ifAlias)
	if err != nil {
		return err
	}

	st.Count.InOctets, err = st.walkUint64(snmp, OID_ifXEntry_ifHCInOctets)
	if err != nil {
		return err
	}

	st.Count.InUcastPkts, err = st.walkUint64(snmp, OID_ifXEntry_ifHCInUcastPkts)
	if err != nil {
		return err
	}

	st.Count.InMulticastPkts, err = st.walkUint64(snmp, OID_ifXEntry_ifHCInMulticastPkts)
	if err != nil {
		return err
	}

	st.Count.InBroadcastPkts, err = st.walkUint64(snmp, OID_ifXEntry_ifHCInBroadcastPkts)
	if err != nil {
		return err
	}

	st.Count.OutOctets, err = st.walkUint64(snmp, OID_ifXEntry_ifHCOutOctets)
	if err != nil {
		return err
	}

	st.Count.OutUcastPkts, err = st.walkUint64(snmp, OID_ifXEntry_ifHCOutUcastPkts)
	if err != nil {
		return err
	}

	st.Count.OutMulticastPkts, err = st.walkUint64(snmp, OID_ifXEntry_ifHCOutMulticastPkts)
	if err != nil {
		return err
	}

	st.Count.OutBroadcastPkts, err = st.walkUint64(snmp, OID_ifXEntry_ifHCOutBroadcastPkts)
	if err != nil {
		return err
	}

	st.Duration = time.Since(st.Timestamp)

	return nil
}

func (st *IfStats) walkName(snmp *gosnmp.GoSNMP) error {
	results, err := snmp.BulkWalkAll(OID_ifXEntry_ifName)
	if err != nil {
		return err
	}

	n := len(results)
	st.State.Index = make([]uint64, n)
	st.State.Name = make([]string, n)

	for i, pdu := range results {
		if pdu.Type != gosnmp.OctetString {
			return fmt.Errorf("Expected OctetString, got %s: %+v", pdu.Type, pdu)
		}

		tail := pdu.Name[len(OID_ifXEntry_ifName)+1:]

		idx, err := strconv.ParseUint(tail, 10, 64)
		if err != nil {
			return err
		}

		b := pdu.Value.([]byte)

		st.State.Index[i] = idx
		st.State.Name[i] = string(b)
	}

	return nil
}

func (st *IfStats) walkString(snmp *gosnmp.GoSNMP, name string) ([]string, error) {
	results, err := snmp.BulkWalkAll(name)
	if err != nil {
		return nil, err
	}

	walk := make([]string, len(results))

	for i, pdu := range results {
		if pdu.Type != gosnmp.OctetString {
			return nil, fmt.Errorf("Expected OctetString, got %s: %+v", pdu.Type, pdu)
		}

		tail := pdu.Name[len(name)+1:]

		idx, err := strconv.ParseUint(tail, 10, 64)
		if err != nil {
			return nil, err
		}

		if idx != st.State.Index[i] {
			return nil, errorsIndexDrift
		}

		b := pdu.Value.([]byte)
		walk[i] = string(b)
	}

	return walk, nil
}

func (st *IfStats) walkBool(snmp *gosnmp.GoSNMP, name string) ([]bool, error) {
	results, err := snmp.BulkWalkAll(name)
	if err != nil {
		return nil, err
	}

	walk := make([]bool, len(results))

	for i, pdu := range results {
		tail := pdu.Name[len(name)+1:]

		idx, err := strconv.ParseUint(tail, 10, 64)
		if err != nil {
			return nil, err
		}

		if idx != st.State.Index[i] {
			return nil, errorsIndexDrift
		}

		var b bool

		switch gosnmp.ToBigInt(pdu.Value).Uint64() {
		case SNMPv2_TC_TruthValue_True:
			b = true

		case SNMPv2_TC_TruthValue_False:
			b = false
		}

		walk[i] = b
	}

	return walk, nil
}

func (st *IfStats) walkUint64(snmp *gosnmp.GoSNMP, name string) ([]uint64, error) {
	results, err := snmp.BulkWalkAll(name)
	if err != nil {
		return nil, err
	}

	walk := make([]uint64, len(results))

	for i, pdu := range results {
		tail := pdu.Name[len(name)+1:]

		idx, err := strconv.ParseUint(tail, 10, 64)
		if err != nil {
			return nil, err
		}

		if idx != st.State.Index[i] {
			return nil, errorsIndexDrift
		}

		val := gosnmp.ToBigInt(pdu.Value).Uint64()
		walk[i] = val
	}

	return walk, nil
}

package calcula

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/soniah/gosnmp"
)

var (
	errorsIndexDrift = errors.New("Inconsistent index")
)

const (
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
	Index   []uint64 `json:"index"`
	Name    []string `json:"name"`
	Alias   []string `json:"alias"`
	Present []bool   `json:"present"`

	Linkspeed        []uint64 `json:"linkspeed"`
	InOctets         []uint64 `json:"in_bytes"`
	InUcastPkts      []uint64 `json:"in_upkts"`
	InMulticastPkts  []uint64 `json:"in_umpkts"`
	InBroadcastPkts  []uint64 `json:"in_bpkts"`
	OutOctets        []uint64 `json:"out_bytes"`
	OutUcastPkts     []uint64 `json:"out_upkts"`
	OutMulticastPkts []uint64 `json:"out_mpkts"`
	OutBroadcastPkts []uint64 `json:"out_bpkts"`
}

func (st *IfStats) Walk(snmp *gosnmp.GoSNMP) error {
	var err error

	err = st.walkName(snmp)
	if err != nil {
		return err
	}

	st.Linkspeed, err = st.walkUint64(snmp, OID_ifXEntry_ifHighSpeed)
	if err != nil {
		return err
	}

	st.Present, err = st.walkBool(snmp, OID_ifXEntry_ifConnectorPresent)
	if err != nil {
		return err
	}

	st.Alias, err = st.walkString(snmp, OID_ifXEntry_ifAlias)
	if err != nil {
		return err
	}

	st.InOctets, err = st.walkUint64(snmp, OID_ifXEntry_ifHCInOctets)
	if err != nil {
		return err
	}

	st.InUcastPkts, err = st.walkUint64(snmp, OID_ifXEntry_ifHCInUcastPkts)
	if err != nil {
		return err
	}

	st.InMulticastPkts, err = st.walkUint64(snmp, OID_ifXEntry_ifHCInMulticastPkts)
	if err != nil {
		return err
	}

	st.InBroadcastPkts, err = st.walkUint64(snmp, OID_ifXEntry_ifHCInBroadcastPkts)
	if err != nil {
		return err
	}

	st.OutOctets, err = st.walkUint64(snmp, OID_ifXEntry_ifHCOutOctets)
	if err != nil {
		return err
	}

	st.OutUcastPkts, err = st.walkUint64(snmp, OID_ifXEntry_ifHCOutUcastPkts)
	if err != nil {
		return err
	}

	st.OutMulticastPkts, err = st.walkUint64(snmp, OID_ifXEntry_ifHCOutMulticastPkts)
	if err != nil {
		return err
	}

	st.OutBroadcastPkts, err = st.walkUint64(snmp, OID_ifXEntry_ifHCOutBroadcastPkts)
	if err != nil {
		return err
	}

	return nil
}

func (st *IfStats) walkName(snmp *gosnmp.GoSNMP) error {
	results, err := snmp.BulkWalkAll(OID_ifXEntry_ifName)
	if err != nil {
		return err
	}

	n := len(results)
	st.Index = make([]uint64, n)
	st.Name = make([]string, n)

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

		st.Index[i] = idx
		st.Name[i] = string(b)
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

		if idx != st.Index[i] {
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

		if idx != st.Index[i] {
			return nil, errorsIndexDrift
		}

		val := gosnmp.ToBigInt(pdu.Value).Uint64()
		b := false

		// FIXME clarify the enum types
		if val == 1 {
			b = true
		} else if val == 2 {
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

		if idx != st.Index[i] {
			return nil, errorsIndexDrift
		}

		val := gosnmp.ToBigInt(pdu.Value).Uint64()
		walk[i] = val
	}

	return walk, nil
}

package ifmibpoller

import (
	"strconv"
	"strings"

	"github.com/soniah/gosnmp"
)

const OID_ifMIB_ifXEntry = ".1.3.6.1.2.1.31.1.1.1."

const (
	// State
	OID_ifName             = 1
	OID_ifHighSpeed        = 15
	OID_ifConnectorPresent = 17
	OID_ifAlias            = 18

	// Measurement
	OID_ifHCInOctets         = 6
	OID_ifHCInUcastPkts      = 7
	OID_ifHCInMulticastPkts  = 8
	OID_ifHCInBroadcastPkts  = 9
	OID_ifHCOutOctets        = 10
	OID_ifHCOutUcastPkts     = 11
	OID_ifHCOutMulticastPkts = 12
	OID_ifHCOutBroadcastPkts = 13
)

type IfMibWalk map[int][]WalkEntry

const (
	WALK_INT int = iota
	WALK_STRING
)

type WalkEntry struct {
	index  uint64
	kind   int
	ivalue uint64
	svalue string
}

type IfMibWalkOutput struct {
	Index []uint64 `json:"index"`

	IfConnectorPresent []bool   `json:"present"`
	IfName             []string `json:"name"`
	IfAlias            []string `json:"alias"`

	IfHighSpeed          []uint64 `json:"linkspeed"`
	IfHCInOctets         []uint64 `json:"in_bytes"`
	IfHCInUcastPkts      []uint64 `json:"in_upkts"`
	IfHCInMulticastPkts  []uint64 `json:"in_mpkts"`
	IfHCInBroadcastPkts  []uint64 `json:"in_bpkts"`
	IfHCOutOctets        []uint64 `json:"out_bytes"`
	IfHCOutUcastPkts     []uint64 `json:"out_upkts"`
	IfHCOutMulticastPkts []uint64 `json:"out_mpkts"`
	IfHCOutBroadcastPkts []uint64 `json:"out_bpkts"`
}

var subsOfInterest map[int]bool

func init() {
	subsOfInterest = map[int]bool{
		1:  true,
		15: true,
		17: true,
		18: true,

		6:  true,
		7:  true,
		8:  true,
		9:  true,
		10: true,
		11: true,
		12: true,
		13: true,
	}
}

func WalkAgent(snmp *gosnmp.GoSNMP) (*IfMibWalkOutput, error) {
	results, err := snmp.BulkWalkAll(OID_ifMIB_ifXEntry)
	if err != nil {
		return nil, err
	}

	w := make(IfMibWalk)
	w.FromResults(results)
	o := w.MakeOutput()

	return &o, nil
}

func (w IfMibWalk) MakeOutput() (o IfMibWalkOutput) {
	entries, _ := w[OID_ifName]
	o.Index = make([]uint64, len(entries))
	for i, entry := range entries {
		o.Index[i] = entry.index
	}

	o.IfName = w.ExtractStrings(OID_ifName)
	o.IfAlias = w.ExtractStrings(OID_ifAlias)

	o.IfHighSpeed = w.ExtractUint64s(OID_ifHighSpeed)
	o.IfHCInOctets = w.ExtractUint64s(OID_ifHCInOctets)
	o.IfHCInUcastPkts = w.ExtractUint64s(OID_ifHCInUcastPkts)
	o.IfHCInMulticastPkts = w.ExtractUint64s(OID_ifHCInMulticastPkts)
	o.IfHCInBroadcastPkts = w.ExtractUint64s(OID_ifHCInBroadcastPkts)
	o.IfHCOutOctets = w.ExtractUint64s(OID_ifHCOutOctets)
	o.IfHCOutUcastPkts = w.ExtractUint64s(OID_ifHCOutUcastPkts)
	o.IfHCOutMulticastPkts = w.ExtractUint64s(OID_ifHCOutMulticastPkts)
	o.IfHCOutBroadcastPkts = w.ExtractUint64s(OID_ifHCOutBroadcastPkts)

	present := w.ExtractUint64s(OID_ifConnectorPresent)
	o.IfConnectorPresent = make([]bool, len(present))

	for i, enum_value := range present {
		b := false

		if enum_value == 1 {
			b = true
		} else if enum_value == 2 {
			b = false
		}

		o.IfConnectorPresent[i] = b
	}

	return o
}

func (w IfMibWalk) ExtractStrings(sub int) (xs []string) {
	entries, ok := w[sub]
	if !ok {
		return nil
	}

	for _, entry := range entries {
		xs = append(xs, entry.svalue)
	}

	return xs
}

func (w IfMibWalk) ExtractUint64s(sub int) (xs []uint64) {
	entries, ok := w[sub]
	if !ok {
		return nil
	}

	for _, entry := range entries {
		xs = append(xs, entry.ivalue)
	}

	return xs
}

func (w IfMibWalk) FromResults(results []gosnmp.SnmpPDU) {
	for _, pdu := range results {
		name := strings.TrimPrefix(pdu.Name, OID_ifMIB_ifXEntry)

		s := strings.Split(name, ".")

		if len(s) != 2 {
			continue
		}

		sub, _ := strconv.Atoi(s[0])
		idx_, _ := strconv.Atoi(s[1])
		idx := uint64(idx_)

		_, ok := subsOfInterest[sub]
		if !ok {
			continue
		}

		entry := WalkEntry{
			index: idx,
		}

		switch pdu.Type {
		case gosnmp.OctetString:
			b := pdu.Value.([]byte)
			entry.kind = WALK_STRING
			entry.svalue = string(b)

		default:
			val := gosnmp.ToBigInt(pdu.Value).Uint64()
			entry.kind = WALK_INT
			entry.ivalue = val
		}

		w[sub] = append(w[sub], entry)
	}
}

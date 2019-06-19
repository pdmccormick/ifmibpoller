package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/soniah/gosnmp"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"calcula"
)

type PollEntry struct {
	Name      string                   `json:"name"`
	Agent     string                   `json:"agent"`
	Timestamp string                   `json:"timestamp"`
	Duration  float64                  `json:"duration"`
	Table     *calcula.IfMibWalkOutput `json:"table"`
}

func main() {
	var err error

	flag.Usage = func() {
		fmt.Printf("Usage:\n")
		fmt.Printf("   %s\n", filepath.Base(os.Args[0]))
		flag.PrintDefaults()
	}

	var community string
	flag.StringVar(&community, "community", "public", "the community string for agent")

	var name string
	flag.StringVar(&name, "name", "switch", "the name of the agent")

	var host string
	flag.StringVar(&host, "host", "localhost", "the address of the agent")

	var pathfmt string
	default_pathfmt := "data/%Y/%Y-%m/%Y%m%d_%H0000.jsonl"
	flag.StringVar(&pathfmt, "pathfmt", default_pathfmt, "the format of output files. Example: data/%Y/%Y-%m/%Y%m%d_%H0000.jsonl")

	var refresh_period_str string
	default_refresh_period_str := "10"
	flag.StringVar(&refresh_period_str, "refresh_period", default_refresh_period_str, "the refresh period of the poller")

	flag.Parse()

	target := host
	default_snmp_port := 161
	port := default_snmp_port

	hs := strings.SplitN(host, ":", 2)
	if len(hs) > 1 {
		target = hs[0]

		port, err = strconv.Atoi(hs[1])
		if err != nil {
			panic(err)
		}
	}

	var rp int
	rp, err = strconv.Atoi(refresh_period_str)
	//refresh_period_f, err = strconv.ParseFloat(refresh_period_str, 64)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Agent: %s:%d\n", target, port)
	fmt.Printf("Refresh period: %vs\n", rp)

	conn := &gosnmp.GoSNMP{
		Version:   gosnmp.Version2c,
		Target:    target,
		Port:      uint16(port), // 1161,
		Community: community,
		Timeout:   time.Duration(10 * time.Second),
		Retries:   3,
		MaxOids:   gosnmp.MaxOids,
	}

	err = conn.Connect()
	if err != nil {
		fmt.Printf("Connect error: %v\n", err)
		os.Exit(1)
	}
	defer conn.Conn.Close()

	entry := &PollEntry{
		Name:  name,
		Agent: target,
	}

	if port != default_snmp_port {
		entry.Agent = fmt.Sprintf("%s:%d", entry.Agent, port)
	}

	now := time.Now

	//refresh_period := refresh_period_f * time.Second
	refresh_period := time.Duration(rp) * time.Second
	wakeup := now().Add(refresh_period)

	loop := true
	for i := 1; ; i++ {
		err = entry.DoPoll(conn, now, pathfmt)

		if err == nil {
			fmt.Printf("%s %s: Captured sample #%d in %.3fs\n", entry.Timestamp, entry.Name, i, entry.Duration)
		} else {
			fmt.Printf("%s %s: Error while polling agent, missed #%d, took %.3fs: %v\n", entry.Timestamp, entry.Name, i, entry.Duration, err)
			i--
		}

		if !loop {
			break
		}

		next_refresh_period := refresh_period
		for {
			until := wakeup.Sub(now())
			wakeup = wakeup.Add(next_refresh_period)

			if until < 0 {
				next_refresh_period *= 2
				fmt.Printf("Increasing refresh_period to %.3f\n", next_refresh_period.Seconds())
				continue
			}

			time.Sleep(until)
			break
		}
	}
}

func (entry *PollEntry) DoPoll(conn *gosnmp.GoSNMP, now func() time.Time, pathfmt string) error {
	var err error

	start := now()
	entry.Table, err = calcula.WalkAgent(conn)
	stop := now()

	entry.Timestamp = start.Format(time.RFC3339Nano)
	entry.Duration = stop.Sub(start).Seconds()

	if err != nil {
		return err
	}

	data, err := json.Marshal(entry)
	if err != nil {
		panic(err)
	}

	path := strings.Replace(pathfmt, "%Y", fmt.Sprintf("%04d", start.Year()), -1)
	path = strings.Replace(path, "%m", fmt.Sprintf("%02d", start.Month()), -1)
	path = strings.Replace(path, "%d", fmt.Sprintf("%02d", start.Day()), -1)
	path = strings.Replace(path, "%H", fmt.Sprintf("%02d", start.Hour()), -1)

	dirpath := filepath.Dir(path)
	err = os.MkdirAll(dirpath, 0775)
	if err != nil {
		panic(err)
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0664)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	_, err = f.Write(data)
	if err != nil {
		panic(err)
	}

	_, err = f.WriteString("\n")
	if err != nil {
		panic(err)
	}

	return nil
}

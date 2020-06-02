package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.pdmccormick.com/sse"
	"gopkg.in/ini.v1"

	"github.com/pdmccormick/ifmibpoller"
)

const (
	DefaultPathFmt                = "data/%N/%Y/%Y-%m/%Y%m%d_%H0000.jsonl"
	DefaultSNMPCommunity          = "public"
	DefaultAgentRefresh           = 10 * time.Second
	DefaultPostAgentStartCooldown = 1 * time.Second
)

type PollEntry struct {
	Name      string               `json:"name"`
	Address   string               `json:"address"`
	Timestamp string               `json:"timestamp"`
	Duration  float64              `json:"duration"`
	Table     *ifmibpoller.IfStats `json:"table"`
}

type PoolConfig struct {
	pathfmt string
	configs map[string]*ifmibpoller.AgentConfig
}

func (pc *PoolConfig) FromIni(cfg *ini.File) bool {
	pc.configs = make(map[string]*ifmibpoller.AgentConfig)

	section, err := cfg.GetSection("agents")
	if err != nil {
		return false
	}

	if section.HasKey("pathfmt") {
		pc.pathfmt = section.Key("pathfmt").MustString("")
	}

	var (
		defaultCommunity = DefaultSNMPCommunity
		defaultRefresh   = DefaultAgentRefresh
	)

	if section.HasKey("refresh") {
		dur, err := section.Key("refresh").Duration()
		if err == nil {
			defaultRefresh = dur
		}
	}

	if section.HasKey("community") {
		defaultCommunity = section.Key("community").MustString(defaultCommunity)
	}

	for _, subsec := range section.ChildSections() {
		nameKey := subsec.Key("name")
		if nameKey == nil || nameKey.MustString("") == "" {
			log.Printf("section %s missing or empty `name` key, ignoring", subsec.Name())
			continue
		}

		name := nameKey.MustString("")

		config := &ifmibpoller.AgentConfig{
			Community: defaultCommunity,
			Refresh:   defaultRefresh,
		}

		pc.configs[name] = config

		if subsec.HasKey("address") {
			config.Address = subsec.Key("address").MustString("")
		}

		if subsec.HasKey("community") {
			config.Community = subsec.Key("community").MustString("")
		}

		if subsec.HasKey("refresh") {
			dur, err := subsec.Key("refresh").Duration()
			if err == nil {
				config.Refresh = dur
			}
		}
	}

	return true
}

type AgentPool struct {
	pathfmt string
	agents  map[string]*ifmibpoller.Agent
	samples chan *PollEntry
}

func NewAgentPool() *AgentPool {
	pool := &AgentPool{
		agents:  make(map[string]*ifmibpoller.Agent),
		samples: make(chan *PollEntry),
	}

	return pool
}

func (pool *AgentPool) ApplyConfig(cfg *PoolConfig) {
	pool.pathfmt = cfg.pathfmt
	if pool.pathfmt == "" {
		pool.pathfmt = DefaultPathFmt
	}

	for name, config := range cfg.configs {
		agent, found := pool.agents[name]
		if !found {
			agent = ifmibpoller.NewAgent(name)
			agent.Start()
			pool.agents[name] = agent

			samples := make(chan *ifmibpoller.IfStats)
			agent.RegisterSampleListener(samples)

			go pool.runSampler(name, config.Address, samples)

			time.Sleep(DefaultPostAgentStartCooldown)
		}

		agent.Configure(config)
	}
}

func (pool *AgentPool) interpolatePath(name string, ts time.Time) string {
	path := pool.pathfmt

	path = strings.ReplaceAll(path, "%Y", fmt.Sprintf("%04d", ts.Year()))
	path = strings.ReplaceAll(path, "%m", fmt.Sprintf("%02d", ts.Month()))
	path = strings.ReplaceAll(path, "%d", fmt.Sprintf("%02d", ts.Day()))
	path = strings.ReplaceAll(path, "%H", fmt.Sprintf("%02d", ts.Hour()))
	path = strings.ReplaceAll(path, "%N", name)

	return path
}

func (pool *AgentPool) runSampler(name, address string, samples chan *ifmibpoller.IfStats) {
	for i := 1; ; i++ {
		var entry = &PollEntry{
			Name:    name,
			Address: address,
		}

		table, ok := <-samples
		if !ok {
			break
		}

		filename := pool.interpolatePath(name, table.Timestamp)

		entry.Prepare(table)

		go func() {
			pool.samples <- entry
		}()

		err := entry.Save(filename)
		if err != nil {
			log.Printf("%s: error while polling agent, missed #%d, took %.3fs: %v", name, i, entry.Duration, err)
			i--
			continue
		}

		log.Printf("%s: captured sample #%d in %.3fs", name, i, entry.Duration)
	}
}

var (
	iniFlag  = flag.String("ini", "", "path to configuration file")
	httpFlag = flag.String("http", "127.0.0.1:8000", "`addr:port` to bind to")
)

func main() {
	flag.Usage = func() {
		fmt.Printf("Usage:\n")
		fmt.Printf("   %s\n", filepath.Base(os.Args[0]))
		flag.PrintDefaults()
	}

	flag.Parse()

	cfg, err := ini.Load(*iniFlag)
	if err != nil {
		log.Fatalf("Fail to read file: %v", err)
	}

	var bus sse.Broadcaster

	http.HandleFunc("/samples", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s: subscribing to samples", r.RemoteAddr)

		var errc = make(chan error)

		ew, err := sse.EventWriter(w)
		if err != nil {
			log.Printf("%s: event writer error: %s", r.RemoteAddr, err)
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}

		bus.Add(ew, errc)
		defer bus.Remove(ew)

		select {
		case err := <-errc:
			log.Printf("%s: error writing samples: %s", r.RemoteAddr, err)

		case <-r.Context().Done():
			break
		}
	})

	go func() {
		log.Printf("Serving event stream at http://%s/samples", *httpFlag)

		log.Fatal(http.ListenAndServe(*httpFlag, nil))
	}()

	pc := &PoolConfig{}
	pc.FromIni(cfg)

	pool := NewAgentPool()
	pool.ApplyConfig(pc)

	var nextId int

	for {
		select {
		case sample := <-pool.samples:
			var ev = sse.Event{
				Id:    fmt.Sprintf("%d", nextId),
				Event: "ifmib-sample",
				Data:  sample,
			}
			nextId++

			ev.WriteTo(&bus)
		}
	}
}

func (entry *PollEntry) Prepare(table *ifmibpoller.IfStats) {
	entry.Table = table
	entry.Timestamp = table.Timestamp.Format(time.RFC3339Nano)
	entry.Duration = table.Duration.Seconds()
}

func (entry *PollEntry) Save(filename string) error {
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	err = os.MkdirAll(filepath.Dir(filename), 0775)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0664)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(data)
	if err != nil {
		return err
	}

	_, err = f.WriteString("\n")
	if err != nil {
		return err
	}

	return nil
}

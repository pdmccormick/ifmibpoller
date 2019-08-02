package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"calcula"

	"gopkg.in/ini.v1"
)

const (
	DefaultPathFmt                = "data/%N/%Y/%Y-%m/%Y%m%d_%H0000.jsonl"
	DefaultSNMPCommunity          = "public"
	DefaultAgentRefresh           = 10 * time.Second
	DefaultPostAgentStartCooldown = 1 * time.Second
)

type PollEntry struct {
	Name      string           `json:"name"`
	Address   string           `json:"address"`
	Timestamp string           `json:"timestamp"`
	Duration  float64          `json:"duration"`
	Table     *calcula.IfStats `json:"table"`
}

type PoolConfig struct {
	pathfmt string
	configs map[string]*calcula.AgentConfig
}

func (pc *PoolConfig) FromIni(cfg *ini.File) bool {
	pc.configs = make(map[string]*calcula.AgentConfig)

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

		config := &calcula.AgentConfig{
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
	agents  map[string]*calcula.Agent
}

func NewAgentPool() *AgentPool {
	pool := &AgentPool{
		agents: make(map[string]*calcula.Agent),
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
			agent = calcula.MakeAgent(name)
			agent.Start()
			pool.agents[name] = agent

			samples := make(chan *calcula.IfStats)
			agent.RegisterListener(samples)

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

func (pool *AgentPool) runSampler(name, address string, samples chan *calcula.IfStats) {
	entry := &PollEntry{
		Name:    name,
		Address: address,
	}

	for i := 1; ; i++ {
		table, ok := <-samples
		if !ok {
			break
		}

		filename := pool.interpolatePath(name, table.Timestamp)

		err := entry.Save(table, filename)
		if err != nil {
			log.Printf("%s: error while polling agent, missed #%d, took %.3fs: %v", name, i, entry.Duration, err)
			i--
			continue
		}

		log.Printf("%s: captured sample #%d in %.3fs", name, i, entry.Duration)
	}
}

func main() {
	var err error

	flag.Usage = func() {
		fmt.Printf("Usage:\n")
		fmt.Printf("   %s\n", filepath.Base(os.Args[0]))
		flag.PrintDefaults()
	}

	iniFlag := flag.String("ini", "", "path to configuration file")

	flag.Parse()

	cfg, err := ini.Load(*iniFlag)
	if err != nil {
		log.Fatalf("Fail to read file: %v", err)
	}

	pc := &PoolConfig{}
	pc.FromIni(cfg)

	pool := NewAgentPool()
	pool.ApplyConfig(pc)

	<-make(chan bool)

	/*
		agent.Stop()
		agent.UnregisterListener(samples)

		for {
			_, ok := <-samples
			if !ok {
				break
			}
		}
	*/
}

func (entry *PollEntry) Save(table *calcula.IfStats, filename string) error {
	var err error

	entry.Table = table
	entry.Timestamp = table.Timestamp.Format(time.RFC3339Nano)
	entry.Duration = table.Duration.Seconds()

	if err != nil {
		return err
	}

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

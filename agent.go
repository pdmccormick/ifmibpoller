package ifmibpoller

import (
	"log"
	"sync"
	"time"

	"github.com/soniah/gosnmp"
)

const (
	DefaultSNMPPort = 161
)

type AgentConfig struct {
	Address   string
	Community string
	Refresh   time.Duration
}

type Agent struct {
	name string

	mu              sync.RWMutex
	running         bool
	stopping        chan chan bool
	configure       chan *agentConfigReq
	sampleListeners map[chan<- *IfStats]bool
}

type agentConfigReq struct {
	config *AgentConfig
	resp   chan bool
}

func NewAgent(name string) *Agent {
	return &Agent{
		name:      name,
		stopping:  make(chan chan bool),
		configure: make(chan *agentConfigReq),
	}
}

func (a *Agent) Start() bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.running {
		return false
	}

	a.running = true
	go a.loop()

	return true
}

func (a *Agent) Stop() bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.running {
		return false
	}

	a.running = false

	errc := make(chan bool)
	a.stopping <- errc
	b := <-errc

	return b
}

func (a *Agent) Configure(config *AgentConfig) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if !a.running {
		return false
	}

	req := &agentConfigReq{
		config: config,
		resp:   make(chan bool),
	}

	a.configure <- req

	return <-req.resp
}

func (a *Agent) RegisterSampleListener(ch chan<- *IfStats) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.sampleListeners == nil {
		a.sampleListeners = make(map[chan<- *IfStats]bool)
	}

	a.sampleListeners[ch] = true
}

func (a *Agent) UnregisterSampleListener(ch chan<- *IfStats) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.sampleListeners == nil {
		return
	}

	delete(a.sampleListeners, ch)
}

func (a *Agent) loop() {
	defaultSnmp := gosnmp.GoSNMP{
		Version: gosnmp.Version2c,
		Port:    161,
		Timeout: time.Duration(10 * time.Second),
		Retries: 3,
		MaxOids: gosnmp.MaxOids,
	}

	var (
		stopC      chan bool
		sampleTick *time.Ticker
		sampleC    <-chan time.Time
		snmp       *gosnmp.GoSNMP
	)

	running := true
	for running {
		select {
		case stopC = <-a.stopping:
			running = false

		case req := <-a.configure:
			log.Printf("%s: configuring %+v", a.name, req.config)

			tmp := defaultSnmp
			tmp.Target = req.config.Address
			tmp.Community = req.config.Community
			tmp.Port = DefaultSNMPPort

			sampleTick = time.NewTicker(req.config.Refresh)
			sampleC = sampleTick.C

			if snmp != nil {
				snmp.Conn.Close()
			}

			snmp = &tmp
			if err := snmp.Connect(); err != nil {
				log.Printf("%s: connect failed: %s", a.name, err)
				req.resp <- false
			} else {
				req.resp <- true
			}

			a.sample(snmp)

		case <-sampleC:
			a.sample(snmp)
		}
	}

	if stopC != nil {
		stopC <- true
	}
}

func (a *Agent) sample(snmp *gosnmp.GoSNMP) {

	if snmp == nil {
		return
	}

	sample := &IfStats{}
	err := sample.Walk(snmp)
	if err != nil {
		log.Printf("%s: walk error: %s", a.name, err)
		return
	}

	a.mu.RLock()
	defer a.mu.RUnlock()

	for ch := range a.sampleListeners {
		ch := ch
		go func() { ch <- sample }()
	}
}

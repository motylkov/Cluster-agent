// Package master provides master election and coordination logic for cloud-agent.
package master

import (
	"cloud-agent/internal/agenttypes"
	"cloud-agent/internal/comms"
	"context"
	"log"
	"sync"
	"time"
)

const (
	defaultWgAddCount    = 2
	defaultChannelBuffer = 10
)

// Command represents a command to be sent to agents.
type Command struct {
	CommandName string
	Args        []string
}

// Service manages agent coordination and command distribution.
type Service struct {
	statusLogInterval int
	SelfID            string
	active            bool
	agentList         *agenttypes.AgentList
	ticker            *time.Ticker
	ctx               context.Context
	cancel            context.CancelFunc
	channel           chan Command
}

// NewService creates a new Service instance.
func NewService(id string, alist *agenttypes.AgentList, statusLogInterval int) *Service {
	return &Service{
		statusLogInterval: statusLogInterval,
		active:            false,
		SelfID:            id,
		agentList:         alist,
	}
}

// Running reports whether the master service is currently active.
func (s *Service) Running() bool {
	return s.active
}

// run is the main loop for the master service.
func (s *Service) run() {
	log.Println("[MASTER] Start master service")

	wg := &sync.WaitGroup{}
	s.ticker = time.NewTicker(time.Duration(s.statusLogInterval) * time.Second)

	agent := (*s.agentList)[s.SelfID]
	agent.Master = false
	(*s.agentList)[s.SelfID] = agent

	defer s.Stop()

	for {
		select {
		case <-s.ctx.Done():
			wg.Wait()
			return
		case command := <-s.channel:
			wg.Add(defaultWgAddCount)
			s.exec("", command)
			wg.Done()
		case <-s.ticker.C:
			log.Println("ping")
			s.channel <- Command{CommandName: "ping"}
		}
	}
}

// exec executes a command on a specific agent or all agents.
func (s *Service) exec(agentname string, command Command) {
	if agentname != "" {
		agent := (*s.agentList)[agentname]
		if agent.Client == nil {
			client, err := comms.NewAgentClient(agent.Address)
			if err != nil {
				log.Printf("[MASTER] Failed to connect with agent %s: %v", agentname, err)
				return
			}
			agent.Client = client
			(*s.agentList)[agentname] = agent
		}

		log.Println("[MASTER] Send command for "+agentname+": ", command)
		comms.SendAsync(agent.Client, command.CommandName, pingReply)
	} else {
		log.Println("[MASTER] Command for all: ", command)
		for id := range *s.agentList {
			agent := (*s.agentList)[id]
			log.Printf("[MASTER] Checking agent %s: active=%v, address=%s, client=%v", id, agent.Active(), agent.Address, agent.Client != nil)
			if !s.agentList.Active(id) {
				log.Printf("[MASTER] Skipping inactive agent %s", id)
				continue
			}

			if agent.Client == nil {
				client, err := comms.NewAgentClient(agent.Address)
				if err != nil {
					log.Printf("[MASTER] Failed to connect with agent %s: %v", id, err)
					agent.SetErr()
					(*s.agentList)[id] = agent
					continue
				}
				agent.Client = client
				(*s.agentList)[id] = agent
			}

			agentPtr := &agent
			comms.SendAsyncWithErrors(agent.Client, command.CommandName, func(reply *comms.Reply, err error) {
				if err != nil {
					agentPtr.SetErr()
					log.Printf("[MASTER] Ping error for %s (err count: %d): %v", id, agentPtr.ErrorCount(), err)
					if !agentPtr.Active() {
						log.Printf("[MASTER] Marking agent %s as inactive", id)
					}
				} else {
					agentPtr.ClearErr()
					log.Printf("[MASTER] Reply from %s: %s", id, reply.Response)
				}
				(*s.agentList)[id] = *agentPtr
			})
		}
	}
}

// Start starts the master service and returns a command channel.
func (s *Service) Start(ctx context.Context) chan Command {
	s.ctx, s.cancel = context.WithCancel(ctx)
	s.active = true
	s.channel = make(chan Command, defaultChannelBuffer)
	go s.run()
	return s.channel
}

// Stop stops the master service and cleans up resources.
func (s *Service) Stop() {
	log.Println("[MASTER] Stop master service")
	s.ticker.Stop()

	agent := (*s.agentList)[s.SelfID]
	agent.Master = false
	(*s.agentList)[s.SelfID] = agent

	s.active = false
	if s.cancel != nil {
		s.cancel()
	}
	close(s.channel)
	s.channel = nil
}

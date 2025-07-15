// Package master provides master election and coordination logic for cloud-agent.
package master

import (
	"cloud-agent/internal/agents"
	"cloud-agent/internal/comms"
	"cloud-agent/internal/config"
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
	Handler     comms.ReplyCallbackFunc
}

// Service manages agent coordination and command distribution.
type Service struct {
	active    bool
	agentPool *agents.AgentPool
	ticker    *time.Ticker
	ctx       context.Context
	cancel    context.CancelFunc
	channel   chan Command
	Config    *config.Config
}

// NewService creates a new Service instance.
func NewService(cfg *config.Config, apool *agents.AgentPool) *Service {
	return &Service{
		active:    false,
		agentPool: apool,
		Config:    cfg,
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
	s.ticker = time.NewTicker(time.Duration(s.Config.StatusLogInterval) * time.Second)

	agent := s.agentPool.Peer[s.Config.SelfID]
	agent.Master = false
	s.agentPool.Peer[s.Config.SelfID] = agent

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
			log.Printf("[MASTER] Periodical ping from master to all")
			s.channel <- Command{CommandName: "ping"}
		}
	}
}

// exec выполняет указанную команду на конкретном агенте или на всех активных.
func (s *Service) exec(agentname string, command Command) {
	if agentname != "" {
		// Выполняем команду на определённом агенте
		agent := s.agentPool.Peer[agentname]
		if agent.Client == nil {
			client, err := comms.NewAgentClient(agent.Address)
			if err != nil {
				log.Printf("[MASTER] Failed to connect with agent %s: %v", agentname, err)
				return
			}
			agent.Client = client
			s.agentPool.Peer[agentname] = agent
		}

		log.Println("[MASTER] Send command for "+agentname+": ", command)
		comms.SendAsync(agent.Client, command.CommandName, s.Config.SelfID, s.Config.ClusterToken, command.Handler)
	} else {
		// Выполняем команду на всех активных агентах
		log.Println("[MASTER] Command for all: ", command)
		for id := range s.agentPool.Peer {
			agent := s.agentPool.Peer[id]
			log.Printf("[MASTER] Checking agent %s: active=%v, address=%s, client=%v", id, s.agentPool.Active(id), agent.Address, agent.Client != nil)
			if !s.agentPool.Active(id) {
				log.Printf("[MASTER] Skipping inactive agent %s", id)
				continue
			}

			if agent.Client == nil {
				client, err := comms.NewAgentClient(agent.Address)
				if err != nil {
					log.Printf("[MASTER] Failed to connect with agent %s: %v", id, err)
					s.agentPool.SetErr(id)
					s.agentPool.Peer[id] = agent
					continue
				}
				agent.Client = client
				s.agentPool.Peer[id] = agent
			}

			agentPtr := &agent
			comms.SendAsyncWithErrors(agent.Client, command.CommandName, s.Config.SelfID, s.Config.ClusterToken, func(reply *comms.Reply, err error) {
				if err != nil {
					s.agentPool.SetErr(id)
					log.Printf("[MASTER] Command error for %s (err count: %d): %v", id, s.agentPool.ErrorCount(id), err)
					if !s.agentPool.Active(id) {
						log.Printf("[MASTER] Marking agent %s as inactive", id)
					}
				} else {
					s.agentPool.ClearErr(id)
					log.Printf("[MASTER] Reply from %s: %s", id, reply.Response)
				}
				s.agentPool.Peer[id] = *agentPtr
			})
		}
	}
}

// ping отправляет команду ping на выбранный агент или на всех агентов.
func (s *Service) ping(agentname string) {
	command := Command{
		CommandName: "ping",
		//        Handler:     pingReply,
	}
	s.exec(agentname, command)
}

// // pingReply представляет собой callback-обработчик для ping-команды.
// func pingReply(reply *comms.Reply, err error) {
// 	if err != nil {
// 		log.Printf("[PING REPLY ERROR] Error occurred during ping request: %v", err)
// 		return
// 	}
// 	log.Printf("[PING REPLY SUCCESS] Response received: %s", reply.Response)
// }

// // Command описывает общую структуру команды, включающую название и обработчик.
// type Command struct {
// 	CommandName string
// 	Handler     comms.ReplyCallbackFunc
// }

// ReplyCallbackFunc — тип обратного вызова для обработки ответов от агентов.
// type ReplyCallbackFunc func(*comms.Reply, error)

// exec executes a command on a specific agent or all agents.
func (s *Service) exec2(agentname string, command Command) {
	if agentname != "" {
		agent := s.agentPool.Peer[agentname]
		if agent.Client == nil {
			client, err := comms.NewAgentClient(agent.Address)
			if err != nil {
				log.Printf("[MASTER] Failed to connect with agent %s: %v", agentname, err)
				return
			}
			agent.Client = client
			s.agentPool.Peer[agentname] = agent
		}

		log.Println("[MASTER] Send command for "+agentname+": ", command)
		comms.SendAsync(agent.Client, command.CommandName, s.Config.SelfID, s.Config.ClusterToken, pingReply)
	} else {
		log.Println("[MASTER] Command for all: ", command)
		for id := range s.agentPool.Peer {
			agent := s.agentPool.Peer[id]
			log.Printf("[MASTER] Checking agent %s: active=%v, address=%s, client=%v", id, s.agentPool.Active(id), agent.Address, agent.Client != nil)
			if !s.agentPool.Active(id) {
				log.Printf("[MASTER] Skipping inactive agent %s", id)
				continue
			}

			if agent.Client == nil {
				client, err := comms.NewAgentClient(agent.Address)
				if err != nil {
					log.Printf("[MASTER] Failed to connect with agent %s: %v", id, err)
					s.agentPool.SetErr(id)
					s.agentPool.Peer[id] = agent
					continue
				}
				agent.Client = client
				s.agentPool.Peer[id] = agent
			}

			agentPtr := &agent
			comms.SendAsyncWithErrors(agent.Client, command.CommandName, s.Config.SelfID, s.Config.ClusterToken, func(reply *comms.Reply, err error) {
				if err != nil {
					s.agentPool.SetErr(id)
					log.Printf("[MASTER] Ping error for %s (err count: %d): %v", id, s.agentPool.ErrorCount(id), err)
					if !s.agentPool.Active(id) {
						log.Printf("[MASTER] Marking agent %s as inactive", id)
					}
				} else {
					s.agentPool.ClearErr(id)
					log.Printf("[MASTER] Reply from %s: %s", id, reply.Response)
				}
				s.agentPool.Peer[id] = *agentPtr
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

	agent := s.agentPool.Peer[s.Config.SelfID]
	agent.Master = false
	s.agentPool.Peer[s.Config.SelfID] = agent

	s.active = false
	if s.cancel != nil {
		s.cancel()
	}
	close(s.channel)
	s.channel = nil
}

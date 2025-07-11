package master

import (
	"cloud-agent/internal/comms"
	"cloud-agent/internal/types"
	"context"
	"log"
	"sync"
	"time"
)

// Command represents a command to be sent to agents.
type Command struct {
	CommandName string
	Args        []string
}

// MasterService manages agent coordination and command distribution.
type MasterService struct {
	status_log_interval int
	SelfID              string
	active              bool
	agentList           *types.AgentList
	ticker              *time.Ticker
	ctx                 context.Context
	cancel              context.CancelFunc
	channel             chan Command
}

// NewMasterService creates a new MasterService instance.
func NewMasterService(id string, alist *types.AgentList, statusLogInterval int) *MasterService {
	return &MasterService{
		status_log_interval: statusLogInterval,
		active:              false,
		SelfID:              id,
		agentList:           alist,
	}
}

// Running reports whether the master service is currently active.
func (master *MasterService) Running() bool {
	return master.active
}

// run is the main loop for the master service.
func (master *MasterService) run() {
	log.Println("[MASTER] Start master service")

	wg := &sync.WaitGroup{}
	master.ticker = time.NewTicker(time.Duration(master.status_log_interval) * time.Second)

	agent := (*master.agentList)[master.SelfID]
	agent.Master = false
	(*master.agentList)[master.SelfID] = agent

	defer master.Stop()

	for {
		select {
		case <-master.ctx.Done():
			wg.Wait()
			return
		case command := <-master.channel:
			wg.Add(2)
			master.exec("", command)
			wg.Done()
		case <-master.ticker.C:
			log.Println("ping")
			master.channel <- Command{CommandName: "ping"}
		}
	}
}

// exec executes a command on a specific agent or all agents.
func (master *MasterService) exec(agentname string, command Command) {
	if agentname != "" {
		agent := (*master.agentList)[agentname]
		if agent.Client == nil {
			client, err := comms.NewAgentClient(agent.Address)
			if err != nil {
				log.Printf("[MASTER] Failed to connect with agent %s: %v", agentname, err)
				return
			}
			agent.Client = client
			(*master.agentList)[agentname] = agent
		}

		log.Println("[MASTER] Send command for "+agentname+": ", command)
		comms.SendAsync(agent.Client, command.CommandName, pingReply)
	} else {
		log.Println("[MASTER] Command for all: ", command)
		for id := range *master.agentList {
			agent := (*master.agentList)[id]
			log.Printf("[MASTER] Checking agent %s: active=%v, address=%s, client=%v", id, agent.Active(), agent.Address, agent.Client != nil)
			if !master.agentList.Active(id) {
				log.Printf("[MASTER] Skipping inactive agent %s", id)
				continue
			}

			if agent.Client == nil {
				client, err := comms.NewAgentClient(agent.Address)
				if err != nil {
					log.Printf("[MASTER] Failed to connect with agent %s: %v", id, err)
					agent.SetErr()
					(*master.agentList)[id] = agent
					continue
				}
				agent.Client = client
				(*master.agentList)[id] = agent
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
				(*master.agentList)[id] = *agentPtr
			})
		}
	}
}

// Start starts the master service and returns a command channel.
func (master *MasterService) Start(ctx context.Context) chan Command {
	master.ctx, master.cancel = context.WithCancel(ctx)
	master.active = true
	master.channel = make(chan Command, 10)
	go master.run()
	return master.channel
}

// Stop stops the master service and cleans up resources.
func (master *MasterService) Stop() {
	log.Println("[MASTER] Stop master service")
	master.ticker.Stop()
	agent := (*master.agentList)[master.SelfID]
	agent.Master = false
	(*master.agentList)[master.SelfID] = agent
	master.active = false
	if master.cancel != nil {
		master.cancel()
	}
	close(master.channel)
	master.channel = nil
}

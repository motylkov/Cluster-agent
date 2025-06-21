package types

import "net/rpc"

type Agent struct {
	Address      string      // IP:Port
	Client       *rpc.Client // for JSON-RPC connection
	Master       bool        // for future
	active       bool
	errorCounter int
}

func (a *Agent) SetErr() {
	a.errorCounter++
	if a.errorCounter >= 5 {
		a.active = false
	}
}

func (a *Agent) ClearErr() {
	a.errorCounter = 0
	a.active = true
}

func (a *Agent) Active() bool {
	return a.active
}

func (a *Agent) ErrorCount() int {
	return a.errorCounter
}

type AgentList map[string]Agent

func (a AgentList) Add(id string, agent Agent) {
	agent.errorCounter = 0
	agent.active = true
	a[id] = agent
}

func (a AgentList) Del(id string) {
	//todo: delete id from AgentList map
}

func (a AgentList) IsErr(id string, num int) bool {
	agent := a[id]
	if agent.errorCounter >= num {
		return true
	}
	return false
}

func (a AgentList) SetErr(id string) {
	agent := a[id]
	agent.SetErr()
	a[id] = agent
}

func (a AgentList) ClearErr(id string) Agent {
	agent := a[id]
	agent.ClearErr()
	return agent
}

func (a AgentList) Active(id string) bool {
	agent := a[id]
	return agent.Active()
}

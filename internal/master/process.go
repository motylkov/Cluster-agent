package master

import (
	"cloud-agent/internal/comms"
	"log"
)

// pingReply logs the result of a ping command sent to an agent.
func pingReply(reply *comms.Reply, err error) {
	if err != nil {
		log.Println("[MASTER] Ping error: ", err)
	} else {
		log.Printf("[MASTER] Reply from agent: %s", reply.Response)
	}
}

package master

import (
	"agent/internal/comms"
	"log"
)

func pingReply(reply *comms.Reply, err error) {
	if err != nil {
		log.Println("[MASTER] Ping error: ", err)
	} else {
		log.Printf("[MASTER] Reply from agent: %s", reply.Response)
	}
}

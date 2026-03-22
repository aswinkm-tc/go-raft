package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/truecaller/go-raft/pkg/raft"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	// Define what "Work" looks like
	work := func(cmd interface{}) interface{} {
		fmt.Printf(">> STATE MACHINE APPLYING: %v\n", cmd)
		return "Done"
	}

	nodes := make([]*raft.RaftNode, 0)
	// Create 3-node cluster
	for i := 0; i < 3; i++ {
		nodes = append(nodes, raft.NewRaftNode(i, work))
	}

	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			if i != j {
				nodes[i].Peers = append(nodes[i].Peers, nodes[j])
			}
		}
	}

	for _, node := range nodes {
		go node.Run(ctx)
	}

	// Wait for a leader to be elected
	time.Sleep(1 * time.Second)

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				fmt.Println("Shutting down...")
				os.Exit(0)
			case <-ticker.C:
				for _, node := range nodes {
					result, ok := node.DoWork("INSERT INTO users VALUES ('Alice')")
					if ok {
						fmt.Println("Work was applied successfully!", "node", node.ID)
						fmt.Printf("Final Client Result: %v\n", result)
						break
					} else {
						fmt.Println("Node 0 is not the leader, please retry with another node.", "node", node.ID)
					}
				}
			}
		}
	}()
	<-ctx.Done()
}

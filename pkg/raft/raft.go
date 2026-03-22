package raft

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

type State int

const (
	Follower State = iota
	Candidate
	Leader
)

// ApplyFunc is the callback for when a command is safely replicated.
type ApplyFunc func(command interface{}) interface{}

type LogEntry struct {
	Term    int
	Command interface{}
}

type RaftNode struct {
	mu sync.Mutex
	ID int

	// Cluster State
	Peers []*RaftNode
	state State

	// Raft State
	currentTerm int
	votedFor    int
	log         []LogEntry

	// Volatile State
	commitIndex int
	lastApplied int

	// Communication
	heartbeat chan bool
	applyCh   chan LogEntry
	onApply   ApplyFunc
}

func NewRaftNode(id int, applyFn ApplyFunc) *RaftNode {
	return &RaftNode{
		ID:          id,
		state:       Follower,
		votedFor:    -1,
		log:         make([]LogEntry, 0),
		heartbeat:   make(chan bool, 100),
		applyCh:     make(chan LogEntry, 100),
		onApply:     applyFn,
		commitIndex: -1,
		lastApplied: -1,
	}
}

// DoWork is the entry point for the user to submit commands.
func (rn *RaftNode) DoWork(command interface{}) (interface{}, bool) {
	rn.mu.Lock()
	if rn.state != Leader {
		rn.mu.Unlock()
		return nil, false // Not the leader, reject request
	}

	// 1. Append to local log
	entry := LogEntry{Term: rn.currentTerm, Command: command}
	rn.log = append(rn.log, entry)
	currIndex := len(rn.log) - 1
	rn.mu.Unlock()

	fmt.Printf("[Node %d] Leader received work: %v\n", rn.ID, command)

	// 2. Replicate to peers
	var successCount atomic.Int32
	successCount.Store(1)

	var wg sync.WaitGroup

	for _, peer := range rn.Peers {
		wg.Add(1)
		go func(p *RaftNode) {
			defer wg.Done()
			ok, term := p.AppendEntries(rn.currentTerm, rn.ID, currIndex, entry)
			if !ok && term > rn.currentTerm {
				rn.mu.Lock()
				if term > rn.currentTerm {
					rn.currentTerm = term
					rn.state = Follower
					fmt.Printf("[Node %d] Stepping down, discovered higher term %d\n", rn.ID, term)
				}
				rn.mu.Unlock()
			} else if ok {
				successCount.Add(1)
			}
		}(peer)
	}

	// 3. Wait briefly for majority (Simplified for this example)
	time.Sleep(50 * time.Millisecond)

	rn.mu.Lock()
	defer rn.mu.Unlock()
	if successCount.Load() > int32(len(rn.Peers)/2) {
		rn.commitIndex = currIndex
		return rn.onApply(command), true
	}

	return nil, false
}

func (rn *RaftNode) AppendEntries(term int, leaderID int, index int, entry LogEntry) (bool, int) {
	rn.mu.Lock()
	defer rn.mu.Unlock()

	// If leader's term is older, reject
	if term < rn.currentTerm {
		return false, rn.currentTerm
	}

	// Accept leader
	rn.currentTerm = term
	rn.state = Follower
	rn.heartbeat <- true

	// If it's a heartbeat (no new entry), just return success
	if index == -1 {
		return true, term
	}

	// Log Replication Logic
	if len(rn.log) <= index {
		rn.log = append(rn.log, entry)
		fmt.Printf("[Node %d] Appended entry at index %d\n", rn.ID, index)

		// In a real library, we'd apply to state machine here
		// once the Leader tells us the commitIndex moved.
		rn.onApply(entry.Command)
	}

	return true, term
}

func (rn *RaftNode) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			fmt.Printf("Node %d shutting down...\n", rn.ID)
			return
		default:
		}
		rn.mu.Lock()
		state := rn.state
		rn.mu.Unlock()

		switch state {
		case Follower:
			select {
			case <-rn.heartbeat:
				// Reset timeout - we heard from the leader
			case <-time.After(time.Duration(150+rand.Intn(150)) * time.Millisecond):
				rn.startElection()
			}
		case Candidate:
			rn.startElection()
		case Leader:
			rn.sendHeartbeats()
			delay := 50
			if rand.Intn(1000) == 0 { // 10% chance of delay
				delay = 2000
			}
			time.Sleep(time.Duration(delay) * time.Millisecond)
		}
	}
}

func (rn *RaftNode) startElection() {
	rn.mu.Lock()
	rn.state = Candidate
	rn.currentTerm++
	rn.votedFor = rn.ID
	term := rn.currentTerm
	rn.mu.Unlock()

	fmt.Printf("Node %d starting election for Term %d\n", rn.ID, term)

	var votes atomic.Int32
	votes.Store(1)
	for _, peer := range rn.Peers {
		go func(p *RaftNode) {
			if p.requestVote(term, rn.ID) {
				rn.mu.Lock()
				votes.Add(1)
				if votes.Load() > int32(len(rn.Peers)/2) && rn.state == Candidate {
					rn.state = Leader
					fmt.Printf("Node %d became LEADER for Term %d\n", rn.ID, term)
				}
				rn.mu.Unlock()
			}
		}(peer)
	}
}

// requestVote is the "RPC" other nodes call
func (rn *RaftNode) requestVote(term int, candidateID int) bool {
	rn.mu.Lock()
	defer rn.mu.Unlock()

	if term > rn.currentTerm {
		rn.currentTerm = term
		rn.votedFor = candidateID
		return true
	}
	return false
}

func (rn *RaftNode) sendHeartbeats() {
	rn.mu.Lock()
	term := rn.currentTerm
	rn.mu.Unlock()

	for _, peer := range rn.Peers {
		go func(p *RaftNode) {
			success, peerTerm := p.AppendEntries(term, rn.ID, -1, LogEntry{})
			if !success && peerTerm > term {
				rn.mu.Lock()
				if peerTerm > rn.currentTerm {
					rn.currentTerm = peerTerm
					rn.state = Follower
					fmt.Printf("[Node %d] Stepping down from heartbeat, discovered higher term %d\n", rn.ID, peerTerm)
				}
				rn.mu.Unlock()
			}
		}(peer)
	}
}

# go-raft

A simplified implementation of the Raft consensus algorithm in Go. This project demonstrates the core concepts of Raft including leader election, log replication, and distributed consensus in a fault-tolerant system.

## Overview

This implementation provides a basic Raft consensus mechanism that allows multiple nodes to agree on a shared state. It includes the fundamental features of the Raft algorithm: leader election through randomized timeouts, heartbeat mechanisms, and log replication across cluster nodes.

## Features

- **Leader Election**: Automatic leader election using randomized election timeouts to prevent vote splitting
- **Log Replication**: Commands are replicated across all nodes in the cluster using AppendEntries RPC
- **State Machine**: Pluggable state machine application through callbacks
- **Heartbeat Mechanism**: Regular heartbeats maintain leader authority and prevent unnecessary elections
- **Majority Consensus**: Commands are committed only when replicated to a majority of nodes
- **Thread-Safe Operations**: Uses atomic operations and mutexes for safe concurrent access
- **Re-election Support**: Handles leader failures and network issues with automatic re-election

## Architecture

The implementation consists of three main states for each node:

- **Follower**: Passive state that responds to requests from leaders and candidates
- **Candidate**: Transition state during leader election
- **Leader**: Handles all client requests and replicates log entries to followers

## Getting Started

### Prerequisites

- Go 1.25.0 or higher

### Installation

```bash
git clone https://github.com/aswinkm-tc/go-raft.git
cd go-raft
```

### Running the Example

The included example demonstrates a 3-node Raft cluster:

```bash
go run main.go
```

This will:
1. Create a 3-node Raft cluster
2. Automatically elect a leader
3. Submit work commands to the cluster
4. Replicate and apply commands across all nodes

### Usage

Create a Raft node with a custom state machine function:

```go
work := func(cmd interface{}) interface{} {
    fmt.Printf("Applying command: %v\n", cmd)
    return "Done"
}

node := raft.NewRaftNode(0, work)
```

Connect nodes in a cluster:

```go
nodes := make([]*raft.RaftNode, 3)
for i := 0; i < 3; i++ {
    nodes[i] = raft.NewRaftNode(i, work)
}

// Connect peers
for i := 0; i < 3; i++ {
    for j := 0; j < 3; j++ {
        if i != j {
            nodes[i].Peers = append(nodes[i].Peers, nodes[j])
        }
    }
}
```

Start the cluster:

```go
ctx := context.Background()
for _, node := range nodes {
    go node.Run(ctx)
}
```

Submit commands to the cluster:

```go
result, ok := node.DoWork("INSERT INTO users VALUES ('Alice')")
if ok {
    fmt.Printf("Command applied: %v\n", result)
} else {
    fmt.Println("Node is not the leader, retry with another node")
}
```

## Project Structure

```
.
├── main.go           # Example usage and demo
├── go.mod           # Go module definition
└── pkg/
    └── raft/
        └── raft.go  # Core Raft implementation
```

## Implementation Details

### Core Components

- **RaftNode**: The main structure representing a node in the Raft cluster
  - Uses `sync.Mutex` for protecting shared state
  - Uses `atomic.Int32` for lock-free counters in vote counting and replication
- **LogEntry**: Represents a single entry in the replicated log with term and command
- **ApplyFunc**: Callback function type for applying committed commands to the state machine
- **State**: Enum representing node state (Follower, Candidate, or Leader)

### Key Methods

- `NewRaftNode(id, applyFn)`: Creates a new Raft node with a given ID and state machine callback
- `Run(ctx)`: Main loop that handles state transitions and timeouts
- `DoWork(command)`: Entry point for submitting commands (only succeeds on the leader)
- `AppendEntries(...)`: RPC-like method for log replication and heartbeats
- `requestVote(...)`: RPC-like method for requesting votes during elections
- `startElection()`: Initiates leader election by transitioning to Candidate state
- `sendHeartbeats()`: Sends periodic heartbeats to maintain leadership

### Concurrency and Thread Safety

The implementation uses several techniques to ensure thread safety:

- **Mutex protection**: The `RaftNode.mu` mutex protects all shared state including term, votedFor, and log
- **Atomic counters**: Vote counting and replication success tracking use `atomic.Int32` to avoid race conditions
- **Buffered channels**: Heartbeat channel has a buffer (100) to prevent blocking
- **Goroutine-per-RPC**: Each RPC call runs in its own goroutine for parallelism

### Timing Parameters

- **Election timeout**: 150-300ms (randomized to prevent split votes)
- **Heartbeat interval**: 50ms base (with occasional delays to simulate network issues)
- **Replication wait**: 50ms timeout for collecting majority responses

## Limitations

This is a simplified educational implementation and is **not production-ready**. Notable simplifications include:

- No persistent storage (all state is in-memory)
- No log compaction or snapshotting
- Simplified timing and synchronization
- No network layer (uses direct method calls instead of RPC)
- Limited error handling and edge case coverage
- No membership changes or cluster reconfiguration
- Stale leader detection may be incomplete (leaders don't check term on AppendEntries responses)
- Log consistency checks are simplified (no prevLogIndex/prevLogTerm validation)
- Commands are applied immediately rather than waiting for commit confirmation
- No retry mechanism for failed RPCs

## Learning Resources

To learn more about the Raft consensus algorithm:

- [Raft Paper](https://raft.github.io/raft.pdf) - "In Search of an Understandable Consensus Algorithm"
- [Raft Visualization](https://raft.github.io/) - Interactive visualization of the Raft algorithm
- [The Secret Lives of Data](http://thesecretlivesofdata.com/raft/) - Visual explanation of Raft
- [Raft Q&A](https://thesquareplanet.com/blog/students-guide-to-raft/) - Students' Guide to Raft

## Development

### Code Quality Tools

Run static analysis:
```bash
go vet ./...
```

Build with race detector:
```bash
go build -race .
```

Run with race detector:
```bash
go run -race main.go
```

### Code Review

See `CODE_REVIEW.md` for a detailed analysis of the implementation, including identified bugs, race conditions, and areas for improvement.

## License

This project is available for educational purposes.

## Contributing

This is a simplified implementation for learning purposes. Contributions that improve clarity or add educational value are welcome. Please note the known limitations and focus on educational improvements rather than production features.

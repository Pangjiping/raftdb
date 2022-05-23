package store

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Pangjiping/raftdb/config"

	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb/v2"
)

// Store is a simple key-value store,
// where all changes are made via raft.
type Store struct {
	mu     sync.Mutex
	m      map[string]string // The key value store for the system.
	config config.Config

	raft   *raft.Raft // The consensus mechanism.
	logger *log.Logger

	RaftDir  string
	RaftBind string
}

func NewStore() *Store {
	store := &Store{
		mu:     sync.Mutex{},
		m:      make(map[string]string),
		logger: log.New(os.Stderr, " [store] ", log.LstdFlags),
	}

	config, err := config.LoadConfig("$GOPATH/src/github.com/Pangjiping/raftdb")
	if err != nil {
		store.logger.Fatalf("Failed to load config: %v", err)
	}
	store.config = config
	return store
}

func (store *Store) LeaderAddr() string {
	return string(store.raft.Leader())
}

// LeaderID returns the node ID of raft leader.
// Returns a blank string if there is no leader, or an error.
func (store *Store) LeaderID() (string, error) {
	addr := store.LeaderAddr()
	configFuture := store.raft.GetConfiguration()
	if err := configFuture.Error(); err != nil {
		store.logger.Printf("Failed to get raft configuration: %v", err)
		return "", err
	}

	for _, srv := range configFuture.Configuration().Servers {
		if srv.Address == raft.ServerAddress(addr) {
			return string(srv.ID), nil
		}
	}
	return "", nil
}

func (store *Store) LeaderAPIAddr() string {
	id, err := store.LeaderID()
	if err != nil {
		return ""
	}

	addr, err := store.GetMeta(id)
	if err != nil {
		return ""
	}
	return addr
}

// Open opens the store.
// If enableSingle is set, and there are no existing peers,
// then this node becomes the first node, and therefore leader, of the cluster.
// localID should be the server identifier for this node.
func (store *Store) Open(enableSingle bool, localID string) error {
	// setup raft configuration
	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID(localID)

	newNode := !pathExists(filepath.Join(store.RaftDir, "raft.db"))

	// setup raft communication.
	addr, err := net.ResolveTCPAddr("tcp", store.RaftBind)
	if err != nil {
		return err
	}
	transport, err := raft.NewTCPTransport(store.RaftBind, addr, store.config.TransportMaxPool, store.config.TransportTimeout, os.Stderr)
	if err != nil {
		return err
	}

	// create the snapshot store.
	// this allows the raft to truncate the log.
	snapshots, err := raft.NewFileSnapshotStore(store.RaftDir, store.config.RetainSnapshotCount, os.Stderr)
	if err != nil {
		return fmt.Errorf("File snapshot store: %s", err)
	}

	// create the log store and stable store.
	var logStore raft.LogStore
	var stableStore raft.StableStore

	boltDB, err := raftboltdb.NewBoltStore(filepath.Join(store.RaftDir, "raft.db"))
	if err != nil {
		return fmt.Errorf("New bolt store: %s", err)
	}
	logStore = boltDB
	stableStore = boltDB

	// instantiate the raft system.
	ra, err := raft.NewRaft(config, (*fsm)(store), logStore, stableStore, snapshots, transport)
	if err != nil {
		return fmt.Errorf("New raft: %s", err)
	}
	store.raft = ra

	if enableSingle && newNode {
		store.logger.Printf("bootstrap needed")
		configuration := raft.Configuration{
			Servers: []raft.Server{
				{
					ID:      config.LocalID,
					Address: transport.LocalAddr(),
				},
			},
		}
		ra.BootstrapCluster(configuration)
	} else {
		store.logger.Printf("no bootstrap needed")
	}
	return nil
}

// WaitForLeader blocks until a leader is detected, or the timeout expires.
func (store *Store) WaitForLeader(timeout time.Duration) (string, error) {
	tck := time.NewTicker(store.config.LeaderWaitDelay)
	defer tck.Stop()
	tmr := time.NewTimer(timeout)
	defer tmr.Stop()

	for {
		select {
		case <-tck.C:
			l := store.LeaderAddr()
			if l != "" {
				return l, nil
			}
		case <-tmr.C:
			return "", fmt.Errorf("timeout expired")
		}
	}
}

// WaitForAppliedIndex blocks until a given log index has been applied.
// or the timeout expires.
func (store *Store) WaitForAppliedIndex(idx uint64, timeout time.Duration) error {
	tck := time.NewTicker(store.config.AppliedWaitDelay)
	defer tck.Stop()
	tmr := time.NewTimer(timeout)
	defer tmr.Stop()

	for {
		select {
		case <-tck.C:
			if store.raft.AppliedIndex() >= idx {
				return nil
			}
		case <-tmr.C:
			return fmt.Errorf("timeout expired")
		}
	}
}

// WaitForApplied waits for all raft log entries to be applied to the
// underlying database.
func (store *Store) WaitForApplied(timeout time.Duration) error {
	if timeout == 0 {
		return nil
	}
	store.logger.Printf("waiting for up to %s for applicaton of initial logs", timeout)
	if err := store.WaitForAppliedIndex(store.raft.LastIndex(), timeout); err != nil {
		return ErrOpenTimeout
	}
	return nil
}

// consistentRead is used to ensure we do not perform a stale
// read. This is done by verifying leadership before the read.
func (store *Store) consistentRead() error {
	future := store.raft.VerifyLeader()
	if err := future.Error(); err != nil {
		return err
	}
	return nil
}

// Get returns the value for the given key.
func (store *Store) Get(key string, lvl ConsistencyLevel) (string, error) {
	if lvl != Stale {
		if store.raft.State() != raft.Leader {
			return "", raft.ErrNotLeader
		}
	}

	if lvl == Consistent {
		if err := store.consistentRead(); err != nil {
			return "", err
		}
	}

	store.mu.Lock()
	defer store.mu.Unlock()
	return store.m[key], nil
}

// Set sets the value for the given key.
func (store *Store) Set(key, value string) error {
	if store.raft.State() != raft.Leader {
		return raft.ErrNotLeader
	}

	c := &command{
		Op:    "set",
		Key:   key,
		Value: value,
	}
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}

	f := store.raft.Apply(b, store.config.RaftTimeout)
	return f.Error()
}

// Delete deletes the given key.
func (store *Store) Delete(key string) error {
	if store.raft.State() != raft.Leader {
		return raft.ErrNotLeader
	}

	c := &command{
		Op:  "delete",
		Key: key,
	}
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}

	f := store.raft.Apply(b, store.config.RaftTimeout)
	return f.Error()
}

func (store *Store) SetMeta(key, value string) error {
	return store.Set(key, value)
}

func (store *Store) GetMeta(key string) (string, error) {
	return store.Get(key, Stale)
}

func (store *Store) DeleteMeta(key string) error {
	return store.Delete(key)
}

// Join joins a node, identified by nodeID and located at addr, to this store.
// The node must be ready to respond to raft communications at that address.
func (store *Store) Join(nodeID, httpAddr string, addr string) error {
	store.logger.Printf("Received join request for remote node %s at %s", nodeID, addr)

	configFuture := store.raft.GetConfiguration()
	if err := configFuture.Error(); err != nil {
		store.logger.Printf("Filed to get raft configuration: %v", err)
		return err
	}

	for _, srv := range configFuture.Configuration().Servers {
		// if a node already exists with either the joining node's ID or address.
		// that node may need to be removed from the config first.
		if srv.ID == raft.ServerID(nodeID) || srv.Address == raft.ServerAddress(addr) {
			// However if both the ID and the address are the same, the nothing -- not even
			// a join operation -- is needed.
			if srv.Address == raft.ServerAddress(addr) && srv.ID == raft.ServerID(nodeID) {
				store.logger.Printf("Node %s at %s already member of cluster, ignoring join request", nodeID, addr)
				return nil
			}

			future := store.raft.RemoveServer(srv.ID, 0, 0)
			if err := future.Error(); err != nil {
				return fmt.Errorf("Error removing existing node %s at %s: %s", nodeID, addr, err)
			}
		}
	}

	f := store.raft.AddVoter(raft.ServerID(nodeID), raft.ServerAddress(addr), 0, 0)
	if f.Error() != nil {
		return f.Error()
	}

	// set meta info
	if err := store.SetMeta(nodeID, httpAddr); err != nil {
		return err
	}

	store.logger.Printf("Node %s at %s joined successfully", nodeID, addr)
	return nil
}

func pathExists(path string) bool {
	if _, err := os.Lstat(path); err != nil && os.IsNotExist(err) {
		return false
	}
	return true
}

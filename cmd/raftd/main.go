package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/Pangjiping/raftdb/pkg/httpd"
	"github.com/Pangjiping/raftdb/pkg/store"
	"github.com/hashicorp/raft"
)

const (
	DefaultHTTPAddr = "localhost:8091"
	DefaultRaftAddr = "localhost:8089"
)

// Command line parameters
var httpAddr string
var raftAddr string
var joinAddr string
var nodeID string

func init() {
	flag.StringVar(&httpAddr, "haddr", DefaultHTTPAddr, "Set the HTTP bind address")
	flag.StringVar(&raftAddr, "raddr", DefaultRaftAddr, "Set Raft bind address")
	flag.StringVar(&joinAddr, "join", "", "Set join address, if any")
	flag.StringVar(&nodeID, "id", "", "Node ID")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: %s [option] <raft-data-path> \n", os.Args[0])
		flag.PrintDefaults()
	}
}

func main() {
	flag.Parse()

	if flag.NArg() == 0 {
		fmt.Fprintf(os.Stderr, "no raft storage directory specified\n")
		os.Exit(1)
	}

	// Ensure Raft storage exists.
	raftDir := flag.Arg(0)
	if raftDir == "" {
		fmt.Fprintf(os.Stderr, "no raft storage directory specified\n")
		os.Exit(1)
	}
	if err := os.MkdirAll(raftDir, 0700); err != nil {
		log.Fatalf("failed to create dir: %s", err.Error())
	}

	s := store.NewStore()
	s.RaftDir = raftDir
	log.Printf("raftdb snapshots location: %s", s.RaftDir)
	s.RaftBind = raftAddr
	if err := s.Open(joinAddr == "", nodeID); err != nil {
		log.Fatalf("Failed to open store: %s", err.Error())
	}

	// if join was specified, make the join request
	if joinAddr != "" {
		if err := join(joinAddr, httpAddr, raftAddr, nodeID); err != nil {
			log.Fatalf("Failed to join node at %s: %s", joinAddr, err.Error())
		}
	} else {
		log.Println("No join addresses set")
	}

	// wait until the store is in full consensus
	openTimeout := 120 * time.Second
	s.WaitForLeader(openTimeout)
	s.WaitForApplied(openTimeout)

	// this may be a standalone server
	// in that case set its own metadata
	if err := s.SetMeta(nodeID, httpAddr); err != nil && err != raft.ErrNotLeader {
		// non-leader errors are ok, since metadata will then be set through
		// consensus as a result of a join
		// all other errors indicate a problem
		log.Fatalf("Failed to SetMeta at %s: %s", nodeID, err.Error())
	}

	h := httpd.NewService(httpAddr, s)
	if err := h.Start(); err != nil {
		log.Fatalf("Failed to start HTTP service: %s", err.Error())
	}

	log.Println("raftDB started successfully!")

	terminate := make(chan os.Signal, 1)
	signal.Notify(terminate, os.Interrupt)
	<-terminate
	log.Println("raftDB exiting! bye~")
}

func join(joinAddr, httpAddr, raftAddr, nodeID string) error {
	b, err := json.Marshal(map[string]string{
		"httpAddr": httpAddr,
		"raftAddr": raftAddr,
		"id":       nodeID,
	})
	if err != nil {
		return err
	}
	resp, err := http.Post(fmt.Sprintf("http://%s/join", joinAddr), "application-type/json", bytes.NewReader(b))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

package httpd

import "github.com/Pangjiping/raftdb/pkg/store"

type Store interface {
	Get(key string, lvl store.ConsistencyLevel) (string, error)
	Set(key, value string) error
	Delete(key string) error
	Join(nodeID string, httpAddr string, addr string) error
	LeaderAPIAddr() string
	SetMeta(key, value string) error
}

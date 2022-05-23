package httpd

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/Pangjiping/raftdb/pkg/store"
	"github.com/hashicorp/raft"
)

type Service struct {
	listener net.Listener
	store    Store
	addr     string
	logger   *log.Logger
}

// FormRedirect returns the value for the "Location" header for a 301 response.
func (svc *Service) FormRedirect(r *http.Request, host string) string {
	protocol := "http"
	rq := r.URL.RawQuery
	if rq != "" {
		rq = fmt.Sprintf("?%s", rq)
	}
	return fmt.Sprintf("%s://%s%s%s", protocol, host, r.URL.Path, rq)
}

func NewService(addr string, store Store) *Service {
	return &Service{
		addr:   addr,
		store:  store,
		logger: log.New(os.Stderr, " [httpd] ", log.LstdFlags),
	}
}

func (svc *Service) Start() error {
	server := http.Server{
		Handler: svc,
	}

	listener, err := net.Listen("tcp", svc.addr)
	if err != nil {
		return err
	}
	svc.listener = listener
	http.Handle("/", svc)

	go func() {
		err := server.Serve(svc.listener)
		if err != nil {
			svc.logger.Fatalf("HTTP serve: %s", err)
		}
	}()

	return nil
}

func (svc *Service) Close() {
	svc.listener.Close()
}

func (svc *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/key") {
		svc.handleKeyRequest(w, r)
	} else if r.URL.Path == "/join" {
		svc.handleJoin(w, r)
	} else {
		w.WriteHeader(http.StatusNotFound)
	}
}

func (svc *Service) handleJoin(w http.ResponseWriter, r *http.Request) {
	m := map[string]string{}
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if len(m) != 3 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	httpAddr, ok := m["httpAddr"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	raftAddr, ok := m["raftAddr"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	nodeID, ok := m["id"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if err := svc.store.Join(nodeID, httpAddr, raftAddr); err != nil {
		if err == raft.ErrNotLeader {
			leader := svc.store.LeaderAPIAddr()
			if leader == "" {
				http.Error(w, err.Error(), http.StatusServiceUnavailable)
				return
			}
			redirect := svc.FormRedirect(r, leader)
			http.Redirect(w, r, redirect, http.StatusTemporaryRedirect)
			return
		}

		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func level(req *http.Request) (store.ConsistencyLevel, error) {
	q := req.URL.Query()
	lvl := strings.TrimSpace(q.Get("level"))

	switch strings.ToLower(lvl) {
	case "default":
		return store.Default, nil
	case "stale":
		return store.Stale, nil
	case "consistent":
		return store.Consistent, nil
	default:
		return store.Default, nil
	}
}

func (svc *Service) handleKeyRequest(w http.ResponseWriter, r *http.Request) {
	getKey := func() string {
		parts := strings.Split(r.URL.Path, "/")
		if len(parts) != 3 {
			w.WriteHeader(http.StatusBadRequest)
			return ""
		}
		return parts[2]
	}

	switch r.Method {
	case "GET":
		k := getKey()
		if k == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		lvl, err := level(r)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		v, err := svc.store.Get(k, lvl)
		if err != nil {
			if err == raft.ErrNotLeader {
				leader := svc.store.LeaderAPIAddr()
				if leader == "" {
					http.Error(w, err.Error(), http.StatusServiceUnavailable)
					return
				}

				redirect := svc.FormRedirect(r, leader)
				http.Redirect(w, r, redirect, http.StatusTemporaryRedirect)
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		b, err := json.Marshal(map[string]string{k: v})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if _, err := io.WriteString(w, string(b)); err != nil {
			svc.logger.Printf("Failed to WriteString: %v", err)
		}

	case "POST":
		m := map[string]string{}
		if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		for k, v := range m {
			if err := svc.store.Set(k, v); err != nil {
				if err == raft.ErrNotLeader {
					leader := svc.store.LeaderAPIAddr()
					if leader == "" {
						http.Error(w, err.Error(), http.StatusServiceUnavailable)
						return
					}

					redirect := svc.FormRedirect(r, leader)
					http.Redirect(w, r, redirect, http.StatusTemporaryRedirect)
					return
				}

				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}

	case "DELETE":
		k := getKey()
		if k == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if err := svc.store.Delete(k); err != nil {
			if err == raft.ErrNotLeader {
				leader := svc.store.LeaderAPIAddr()
				if leader == "" {
					http.Error(w, err.Error(), http.StatusServiceUnavailable)
					return
				}

				redirect := svc.FormRedirect(r, leader)
				http.Redirect(w, r, redirect, http.StatusTemporaryRedirect)
				return
			}

			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		// svc.store.Delete(k)

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (svc *Service) Addr() net.Addr {
	return svc.listener.Addr()
}

package httpd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/Pangjiping/raftdb/pkg/store"
	"github.com/stretchr/testify/require"
)

func TestNewService(t *testing.T) {
	store := newTestStore()
	require.NotNil(t, store)
	s := &testServer{NewService(":0", store)}

	// start
	err := s.Start()
	require.NoError(t, err)

	// get
	b := doGET(t, s.URL(), "k1")
	require.Equal(t, `{"k1":""}`, string(b))

	// post
	doPOST(t, s.URL(), "k1", "v1")
	b = doGET(t, s.URL(), "k1")
	require.Equal(t, `{"k1":"v1"}`, string(b))

	store.m["k2"] = "v2"
	b = doGET(t, s.URL(), "k2")
	require.Equal(t, `{"k2":"v2"}`, string(b))

	doDELETE(t, s.URL(), "k2")
	b = doGET(t, s.URL(), "k2")
	require.Equal(t, `{"k2":""}`, string(b))
}

type testServer struct {
	*Service
}

func (t *testServer) URL() string {
	port := strings.TrimLeft(t.Addr().String(), "[::]:")
	return fmt.Sprintf("http://127.0.0.1:%s", port)
}

type testStore struct {
	m map[string]string
}

func newTestStore() *testStore {
	return &testStore{
		m: make(map[string]string),
	}
}

func (t *testStore) Get(key string, lvl store.ConsistencyLevel) (string, error) {
	return t.m[key], nil
}

func (t *testStore) Set(key, value string) error {
	t.m[key] = value
	return nil
}

func (t *testStore) Delete(key string) error {
	delete(t.m, key)
	return nil
}

func (t *testStore) Join(nodeID, httpAddr, addr string) error {
	return nil
}

func (t *testStore) LeaderAPIAddr() string {
	return ""
}

func (t *testStore) SetMeta(key, value string) error {
	return nil
}

func doGET(t *testing.T, url, key string) string {
	resp, err := http.Get(fmt.Sprintf("%s/key/%s", url, key))
	if err != nil {
		t.Fatalf("Failed to GET key: %v", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}
	return string(body)
}

func doPOST(t *testing.T, url, key, value string) {
	b, err := json.Marshal(map[string]string{key: value})
	if err != nil {
		t.Fatalf("Failed to encode key and value for POST: %v", err)
	}
	resp, err := http.Post(fmt.Sprintf("%s/key", url), "application-type/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("POST request failed: %v", err)
	}
	defer resp.Body.Close()
}

func doDELETE(t *testing.T, u, key string) {
	ru, err := url.Parse(fmt.Sprintf("%s/key/%s", u, key))
	if err != nil {
		t.Fatalf("Failed to parse URL for delete: %v", err)
	}
	req := &http.Request{
		Method: "DELETE",
		URL:    ru,
	}

	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to GET key: %v", err)
	}
	defer resp.Body.Close()
}

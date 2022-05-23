package store

import (
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestStoreOpen tests that store can be opened
func TestStoreOpen(t *testing.T) {
	s := NewStore()
	require.NotNil(t, s)

	tmpDir, _ := ioutil.TempDir("", "store_test")
	defer os.RemoveAll(tmpDir)

	s.RaftBind = "127.0.0.1:0"
	s.RaftDir = tmpDir

	err := s.Open(false, "node0")
	require.NoError(t, err)
}

func TestStoreOpenSingleNode(t *testing.T) {
	s := NewStore()
	require.NotNil(t, s)

	tmpDir, _ := ioutil.TempDir("", "store_test")
	defer os.RemoveAll(tmpDir)

	s.RaftBind = "127.0.0.1:0"
	s.RaftDir = tmpDir

	err := s.Open(true, "node0")
	require.NoError(t, err)

	// simple way to ensure there is a leader
	time.Sleep(3 * time.Second)

	// set a value
	err = s.Set("foo", "bar")
	require.NoError(t, err)

	// wait for committed log entry to be applied
	time.Sleep(500 * time.Millisecond)
	value, err := s.Get("foo", Default)
	require.NoError(t, err)
	require.Equal(t, "bar", value)

	// delete value
	err = s.Delete("foo")
	require.NoError(t, err)

	// wait for committed log entry to be applied
	time.Sleep(500 * time.Millisecond)
	value, err = s.Get("foo", Default)
	require.NoError(t, err)
	require.Equal(t, "", value)
}

func TestStoreInMemOpenSingleNode(t *testing.T) {
	s := NewStore()
	require.NotNil(t, s)

	tmpDir, _ := ioutil.TempDir("", "store_test")
	defer os.RemoveAll(tmpDir)

	s.RaftBind = "127.0.0.1:0"
	s.RaftDir = tmpDir

	err := s.Open(true, "node0")
	require.NoError(t, err)

	// simple way to ensure there is a leader
	time.Sleep(3 * time.Second)

	err = s.Set("foo", "bar")
	require.NoError(t, err)

	// wait for committed log entry to be applied
	time.Sleep(500 * time.Millisecond)
	value, err := s.Get("foo", Default)
	require.NoError(t, err)
	require.Equal(t, "bar", value)

	err = s.Delete("foo")
	require.NoError(t, err)

	// wait for committed log entry to be applied
	time.Sleep(500 * time.Millisecond)
	value, err = s.Get("foo", Default)
	require.NoError(t, err)
	require.Equal(t, "", value)
}

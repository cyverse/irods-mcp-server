package common

import (
	"fmt"
	"sync"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	gocache "github.com/patrickmn/go-cache"
)

// pendingConn represents an in-flight connection attempt.
// Goroutines that miss the cache for the same key wait on ready instead of
// each starting their own connection.
type pendingConn struct {
	ready chan struct{}
	fs    *irodsclient_fs.FileSystem
	err   error
}

// IRODSFSClientPool caches iRODS filesystem connections per (zone, user).
// Concurrent cache misses for the same key share a single connection attempt;
// misses for different keys proceed in parallel.
type IRODSFSClientPool struct {
	fsclientCache *gocache.Cache
	mu            sync.Mutex
	pending       map[string]*pendingConn
}

func NewIRODSFSClientPool() *IRODSFSClientPool {
	fsclientCache := gocache.New(fsPoolTimeout, fsPoolTimeout)

	fsclientCache.OnEvicted(func(key string, value interface{}) {
		if fsClient, ok := value.(*irodsclient_fs.FileSystem); ok {
			fsClient.Release()
		}
	})

	return &IRODSFSClientPool{
		fsclientCache: fsclientCache,
		pending:       make(map[string]*pendingConn),
	}
}

func fsPoolKey(account *irodsclient_types.IRODSAccount) string {
	return fmt.Sprintf("%s:%s", account.ClientZone, account.ClientUser)
}

func (pool *IRODSFSClientPool) GetIRODSFSClient(account *irodsclient_types.IRODSAccount) (*irodsclient_fs.FileSystem, error) {
	account.FixAuthConfiguration()

	key := fsPoolKey(account)

	pool.mu.Lock()

	// Cache hit — return immediately.
	if obj, ok := pool.fsclientCache.Get(key); ok {
		pool.mu.Unlock()
		if fsClient, ok2 := obj.(*irodsclient_fs.FileSystem); ok2 {
			return fsClient, nil
		}
	}

	// Another goroutine is already connecting for this key — wait for it.
	if p, ok := pool.pending[key]; ok {
		pool.mu.Unlock()
		<-p.ready
		return p.fs, p.err
	}

	// First miss for this key — register in-flight entry and connect outside the lock.
	p := &pendingConn{ready: make(chan struct{})}
	pool.pending[key] = p
	pool.mu.Unlock()

	p.fs, p.err = GetIRODSFSClient(account)

	pool.mu.Lock()
	delete(pool.pending, key)
	if p.err == nil {
		pool.fsclientCache.SetDefault(key, p.fs)
	}
	pool.mu.Unlock()

	close(p.ready) // unblock all waiters
	return p.fs, p.err
}

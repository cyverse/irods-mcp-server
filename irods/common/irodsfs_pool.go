package common

import (
	"sync"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	gocache "github.com/patrickmn/go-cache"
)

type IRODSFSClientPool struct {
	fsclientCache *gocache.Cache // map[string]*irodsclient_fs.FileSystem // username is the key
	mutex         sync.RWMutex
}

func NewIRODSFSClientPool() *IRODSFSClientPool {
	fsclientCache := gocache.New(fsPoolTimeout, fsPoolTimeout)

	// release filesystem when evicted
	fsclientCache.OnEvicted(func(key string, value interface{}) {
		if fsClient, ok := value.(*irodsclient_fs.FileSystem); ok {
			fsClient.Release()
		}
	})

	return &IRODSFSClientPool{
		fsclientCache: fsclientCache,
	}
}

func (pool *IRODSFSClientPool) GetIRODSFSClient(account *irodsclient_types.IRODSAccount) (*irodsclient_fs.FileSystem, error) {
	account.FixAuthConfiguration()

	pool.mutex.RLock()
	fsClientObj, ok := pool.fsclientCache.Get(account.ClientUser)
	pool.mutex.RUnlock()

	if ok {
		if fsClient, ok2 := fsClientObj.(*irodsclient_fs.FileSystem); ok2 {
			return fsClient, nil
		}
	}

	fsClient, err := GetIRODSFSClient(account)
	if err != nil {
		return nil, err
	}

	pool.mutex.Lock()
	defer pool.mutex.Unlock()

	pool.fsclientCache.SetDefault(account.ClientUser, fsClient)

	return fsClient, nil
}

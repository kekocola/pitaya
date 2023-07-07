// Copyright (c) TFG Co. All Rights Reserved.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package modules

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/topfreegames/pitaya/v2/cluster"
	"github.com/topfreegames/pitaya/v2/config"
	"github.com/topfreegames/pitaya/v2/constants"
	"github.com/topfreegames/pitaya/v2/logger"
	"github.com/topfreegames/pitaya/v2/session"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/namespace"
)

var void = struct{}{} // empty instance

// ETCDBindingStorage module that uses etcd to keep in which frontend server each user is bound
type ETCDBindingStorage struct {
	Base
	cli             *clientv3.Client
	etcdEndpoints   []string
	etcdPrefix      string
	etcdDialTimeout time.Duration
	leaseTTL        time.Duration
	leaseID         clientv3.LeaseID
	thisServer      *cluster.Server
	sessionPool     session.SessionPool
	stopChan        chan struct{}
	onlineUsers     map[uint64]struct{}
	running         bool
}

// NewETCDBindingStorage returns a new instance of BindingStorage
func NewETCDBindingStorage(server *cluster.Server, sessionPool session.SessionPool, conf config.ETCDBindingConfig) *ETCDBindingStorage {
	b := &ETCDBindingStorage{
		thisServer:  server,
		sessionPool: sessionPool,
		stopChan:    make(chan struct{}),
		onlineUsers: make(map[uint64]struct{}),
		running:     false,
	}
	b.etcdDialTimeout = conf.DialTimeout
	b.etcdEndpoints = conf.Endpoints
	b.etcdPrefix = conf.Prefix
	b.leaseTTL = conf.LeaseTTL
	return b
}

func getUserBindingKey(uid, frontendType string) string {
	return fmt.Sprintf("bindings/%s/%s", frontendType, uid)
}

func parseBindingsKey(key string) (string, error) {
	splittedUser := strings.Split(key, "/")
	if len(splittedUser) != 3 {
		return "", fmt.Errorf("error parsing bindings key %s", key)
	}
	uid := splittedUser[2]
	return uid, nil
}

// PutBinding puts the binding info into etcd
func (b *ETCDBindingStorage) PutBinding(uid string) error {
	_, err := b.cli.Put(context.Background(), getUserBindingKey(uid, b.thisServer.Type), b.thisServer.ID, clientv3.WithLease(b.leaseID))
	return err
}

func (b *ETCDBindingStorage) removeBinding(uid string) error {
	_, err := b.cli.Delete(context.Background(), getUserBindingKey(uid, b.thisServer.Type))
	return err
}

// GetUserFrontendID gets the id of the frontend server a user is connected to
// TODO: should we set context here?
// TODO: this could be way more optimized, using watcher and local caching
func (b *ETCDBindingStorage) GetUserFrontendID(uid, frontendType string) (string, error) {
	etcdRes, err := b.cli.Get(context.Background(), getUserBindingKey(uid, frontendType))
	if err != nil {
		return "", err
	}
	if len(etcdRes.Kvs) == 0 {
		return "", constants.ErrBindingNotFound
	}
	return string(etcdRes.Kvs[0].Value), nil
}

func (b *ETCDBindingStorage) setupOnSessionCloseCB() {
	b.sessionPool.OnSessionClose(func(s session.Session) {
		if s.UID() != "" {
			err := b.removeBinding(s.UID())
			if err != nil {
				logger.Log.Errorf("error removing binding info from storage: %v", err)
			}
		}
	})
}

func (b *ETCDBindingStorage) setupOnAfterSessionBindCB() {
	b.sessionPool.OnAfterSessionBind(func(ctx context.Context, s session.Session) error {
		return b.PutBinding(s.UID())
	})
}

func (b *ETCDBindingStorage) watchLeaseChan(c <-chan *clientv3.LeaseKeepAliveResponse) {
	for {
		select {
		case <-b.stopChan:
			return
		case kaRes := <-c:
			if kaRes == nil {
				logger.Log.Warn("[binding storage] sd: error renewing etcd lease, rebootstrapping")
				for {
					err := b.bootstrapLease()
					if err != nil {
						logger.Log.Warn("[binding storage] sd: error rebootstrapping lease, will retry in 5 seconds")
						time.Sleep(5 * time.Second)
						continue
					} else {
						return
					}
				}
			}
		}
	}
}

func (b *ETCDBindingStorage) bootstrapLease() error {
	// grab lease
	l, err := b.cli.Grant(context.TODO(), int64(b.leaseTTL.Seconds()))
	if err != nil {
		return err
	}
	b.leaseID = l.ID
	logger.Log.Debugf("[binding storage] sd: got leaseID: %x", l.ID)
	// this will keep alive forever, when channel c is closed
	// it means we probably have to rebootstrap the lease
	c, err := b.cli.KeepAlive(context.TODO(), b.leaseID)
	if err != nil {
		return err
	}
	// need to receive here as per etcd docs
	<-c
	go b.watchLeaseChan(c)
	return nil
}

// Init starts the binding storage module
func (b *ETCDBindingStorage) Init() error {
	var cli *clientv3.Client
	var err error
	if b.cli == nil {
		cli, err = clientv3.New(clientv3.Config{
			Endpoints:   b.etcdEndpoints,
			DialTimeout: b.etcdDialTimeout,
		})
		if err != nil {
			return err
		}
		b.cli = cli
		b.cli.Watcher = namespace.NewWatcher(b.cli.Watcher, b.etcdPrefix)
	}
	// namespaced etcd :)
	b.cli.KV = namespace.NewKV(b.cli.KV, b.etcdPrefix)
	err = b.bootstrapLease()
	if err != nil {
		return err
	}

	if b.thisServer.Frontend {
		b.setupOnSessionCloseCB()
		b.setupOnAfterSessionBindCB()
	}

	b.running = true
	go b.watchUserChange()

	return nil
}

// Shutdown executes on shutdown and will clean etcd
func (b *ETCDBindingStorage) Shutdown() error {
	b.running = false
	close(b.stopChan)
	return b.cli.Close()
}

// add online user
func (b *ETCDBindingStorage) addOnlineUser(uid string) {
	id, err := strconv.ParseUint(uid, 10, 64)
	if err == nil {
		b.onlineUsers[id] = void
	}
}

// delete online user
func (b *ETCDBindingStorage) deleteOnlineUser(uid string) {
	id, err := strconv.ParseUint(uid, 10, 64)
	if err == nil {
		delete(b.onlineUsers, id)
	}
}

// is online user
func (b *ETCDBindingStorage) IsUserOnline(uid uint64) bool {
	_, ok := b.onlineUsers[uid]
	return ok
}

// watch user login or logout
func (b *ETCDBindingStorage) watchUserChange() {
	w := b.cli.Watch(context.Background(), "bindings/", clientv3.WithPrefix())
	go func(chn clientv3.WatchChan) {
		for b.running {
			select {
			case wResp, ok := <-chn:
				if wResp.Err() != nil {
					logger.Log.Warnf("etcd watcher user response error: %s", wResp.Err())
					time.Sleep(1000 * time.Millisecond)
				}
				if !ok {
					logger.Log.Error("etcd watcher user died, retrying to watch in 1 second")
					time.Sleep(1000 * time.Millisecond)
					chn = b.cli.Watch(context.Background(), "bindings/", clientv3.WithPrefix())
					continue
				}
				for _, ev := range wResp.Events {
					uid, err := parseBindingsKey(string(ev.Kv.Key))
					if err != nil {
						logger.Log.Warnf("failed to parse bindings key from etcd: %s", ev.Kv.Key)
						continue
					}

					switch ev.Type {
					case clientv3.EventTypePut:
						b.addOnlineUser(uid)
					case clientv3.EventTypeDelete:
						b.deleteOnlineUser(uid)
					}
				}
			case <-b.stopChan:
				return
			}
		}
	}(w)
}

package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

type stCached struct {
	hs   *httpServer
	opts *options
	log  *log.Logger
	cm   *cacheManager
	raft *raftNodeInfo
}

type stCachedContext struct {
	st *stCached
}

func main() {
	st := &stCached{
		opts: NewOptions(),
		log:  log.New(os.Stderr, "stCached: ", log.Ldate|log.Ltime),
		cm:   NewCacheManager(),
	}
	ctx := &stCachedContext{st}

	//var l net.Listener
	//var err error
	//l, err = net.Listen("tcp", st.opts.httpAddress)
	//if err != nil {
	//	st.log.Fatal(fmt.Sprintf("listen %s failed: %s", st.opts.httpAddress, err))
	//}
	//st.log.Printf("http server listen:%s", l.Addr())

	logger := log.New(os.Stderr, "httpserver: ", log.Ldate|log.Ltime)
	httpServer := NewHttpServer(ctx, logger)
	st.hs = httpServer
	go func() {
		http.ListenAndServe(st.opts.httpAddress, httpServer.mux)
		// http.Serve(l, httpServer.mux)
	}()

	raft, err := newRaftNode(st.opts, ctx)
	if err != nil {
		st.log.Fatal(fmt.Sprintf("new raft node failed:%v", err))
	}
	st.raft = raft

	if st.opts.joinAddress != "" {
		err = joinRaftCluster(st.opts)
		if err != nil {
			st.log.Fatal(fmt.Sprintf("join raft cluster failed:%v", err))
		}
	}

	// monitor leadership
	for {
		select {
		//  当故障切换的时候，follower变成了leader，应用程序如何感知到呢？
		//  在raft结构里面提供有一个eaderCh，它是bool类型的channel，不带缓存，
		//  当本节点的leader状态有变化的时候，会往这个channel里面写数据，
		//  但是由于不带缓冲且写数据的协程不会阻塞在这里，有可能会写入失败，
		//  没有及时变更状态，所以使用leaderCh的可靠性不能保证。
		//  好在raft Config里面提供了另一个channel NotifyCh，它是带缓存的，
		//  当leader状态变化时会往这个chan写数据，写入的变更消息能够缓存在channel里面 ，
		//  应用程序能够通过它获取到最新的状态变化。
		case leader := <-st.raft.leaderNotifyCh:
			if leader {
				st.log.Println("become leader, enable write api")
				// 修改标志
				st.hs.setWriteFlag(true)
			} else {
				st.log.Println("become follower, close write api")
				st.hs.setWriteFlag(false)
			}
		}
	}
}

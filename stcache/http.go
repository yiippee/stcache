package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/hashicorp/raft"
)

const (
	ENABLE_WRITE_TRUE  = int32(1)
	ENABLE_WRITE_FALSE = int32(0)
)

type httpServer struct {
	ctx *stCachedContext
	log *log.Logger
	mux *http.ServeMux
	// leader选举是属于raft协议的内容，不需要应用程序操心，
	// 但是对有些场景而言，应用程序需要感知leader状态，
	// 比如对stcache而言，理论上只有leader才能处理set请求来写数据，follower应该只能处理get请求查询数据。
	// 为了模拟说明这个情况，我们在stcache里面我们设置一个写标志位，
	// 当本节点是leader的时候标识位置true，可以处理set请求，
	// 否则标识位为false，不能处理set请求。
	enableWrite int32
}

func Test(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "%s", "hello,world.")
}
func NewHttpServer(ctx *stCachedContext, log *log.Logger) *httpServer {
	mux := http.NewServeMux()
	s := &httpServer{
		ctx:         ctx,
		log:         log,
		mux:         mux,
		enableWrite: ENABLE_WRITE_FALSE,
	}
	mux.HandleFunc("/", Test)
	mux.HandleFunc("/set", s.doSet)
	mux.HandleFunc("/get", s.doGet)
	mux.HandleFunc("/join", s.doJoin)
	mux.HandleFunc("/list", s.list)
	return s
}

func (h *httpServer) checkWritePermission() bool {
	return atomic.LoadInt32(&h.enableWrite) == ENABLE_WRITE_TRUE
}

func (h *httpServer) setWriteFlag(flag bool) {
	if flag {
		atomic.StoreInt32(&h.enableWrite, ENABLE_WRITE_TRUE)
	} else {
		atomic.StoreInt32(&h.enableWrite, ENABLE_WRITE_FALSE)
	}
}

func (h *httpServer) list(w http.ResponseWriter, r *http.Request) {
	for k, v := range h.ctx.st.cm.data {
		fmt.Fprintf(w, "%s : %s\n", k, v)
	}
}

func (h *httpServer) doGet(w http.ResponseWriter, r *http.Request) {
	vars := r.URL.Query()

	key := vars.Get("key")
	if key == "" {
		h.log.Println("doGet() error, get nil key")
		fmt.Fprint(w, "")
		return
	}

	ret := h.ctx.st.cm.Get(key)
	fmt.Fprintf(w, "%s\n", ret)
}

// doSet saves data to cache, only raft master node provides this api
func (h *httpServer) doSet(w http.ResponseWriter, r *http.Request) {
	if !h.checkWritePermission() {
		fmt.Fprint(w, "write method not allowed\n")
		return
	}
	vars := r.URL.Query()

	key := vars.Get("key")
	value := vars.Get("value")
	if key == "" || value == "" {
		h.log.Println("doSet() error, get nil key or nil value")
		fmt.Fprint(w, "param error\n")
		return
	}

	event := logEntryData{Key: key, Value: value}
	eventBytes, err := json.Marshal(event)
	if err != nil {
		h.log.Printf("json.Marshal failed, err:%v", err)
		fmt.Fprint(w, "internal error\n")
		return
	}

	//  这里不再直接写缓存，而是调用raft的Apply方式，为这次set操作生成一个log entry，
	//  这里面会根据raft的内部协议，在各个节点之间进行通信协作，
	//  确保最后这条log 会在整个集群的节点里面提交或者失败。

	// 对follower节点来说，leader会通知它来commit log entry，
	// 被commit的log entry需要调用应用层提供的Apply方法来执行日志，
	// 这里就是从logEntry拿到具体的数据，然后写入缓存里面即可。
	applyFuture := h.ctx.st.raft.raft.Apply(eventBytes, 5*time.Second) // 5s 会不会严重影响程序的响应时间啊？应该不会，反正休眠时不占cpu
	// Apply 返回一个future，一个future代表一个已经完成或者未完成的操作
	// 判定future的状态，确定执行的最终结果
	if err := applyFuture.Error(); err != nil {
		h.log.Printf("raft.Apply failed:%v", err)
		fmt.Fprint(w, "internal error\n")
		return
	}

	fmt.Fprintf(w, "ok\n")
}

// doJoin handles joining cluster request
// 后续的节点启动的时候需要加入集群，启动的时候指定第一个节点的地址，并发送请求加入集群，
// 这里我们定义成直接通过http请求。
func (h *httpServer) doJoin(w http.ResponseWriter, r *http.Request) {
	vars := r.URL.Query()

	// 获取对方的地址（指raft集群内部通信的tcp地址）
	peerAddress := vars.Get("peerAddress")
	if peerAddress == "" {
		h.log.Println("invalid PeerAddress")
		fmt.Fprint(w, "invalid peerAddress\n")
		return
	}
	// 调用AddVoter把这个节点加入到集群即可。申请加入的节点会进入follower状态，
	// 这以后集群节点之间就可以正常通信，leader也会把数据同步给follower。
	addPeerFuture := h.ctx.st.raft.raft.AddVoter(raft.ServerID(peerAddress), raft.ServerAddress(peerAddress), 0, 0)
	if err := addPeerFuture.Error(); err != nil {
		h.log.Printf("Error joining peer to raft, peeraddress:%s, err:%v, code:%d", peerAddress, err, http.StatusInternalServerError)
		fmt.Fprint(w, "internal error\n")
		return
	}
	fmt.Fprint(w, "ok")
}

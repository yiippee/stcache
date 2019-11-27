package main

import (
	"encoding/json"
	"io"
	"log"

	"github.com/hashicorp/raft"
)

type FSM struct {
	ctx *stCachedContext
	log *log.Logger
}

type logEntryData struct {
	Key   string
	Value string
}

// Apply applies a Raft log entry to the key-value store.
// 当raft内部commit了一个log entry后，会记录在上面说过的logStore里面，
// 被commit的log entry需要被执行，就stcache来说，
// 执行log entry就是把数据写入缓存，即执行set操作

// 对follower节点来说，leader会通知它来commit log entry，
// 被commit的log entry需要调用应用层提供的Apply方法(也就是下面的这个Apply方法)来执行日志，
// 这里就是从logEntry拿到具体的数据，然后写入缓存里面即可。

// 如果这条日志被一半以上的follew成功的复制，领导人就应用这条日志到自己的状态机（就是下面的Apply函数，将log entry作用到本地状态机）中，并返回给客户端。
// 所以需要用户自己实现fsm啊。如果 follower 宕机或者运行缓慢或者丢包，领导人会不断的重试（会永远重试吗？），直到所有的 follower 最终都存储了所有的日志条目。
func (f *FSM) Apply(logEntry *raft.Log) interface{} {
	e := logEntryData{}
	if err := json.Unmarshal(logEntry.Data, &e); err != nil {
		panic("Failed unmarshaling Raft log entry. This is a bug.")
	}
	ret := f.ctx.st.cm.Set(e.Key, e.Value)
	f.log.Printf("fms.Apply(), logEntry:%s, ret:%v\n", logEntry.Data, ret)
	return ret
}

// Snapshot returns a latest snapshot
// 生成一个快照结构
func (f *FSM) Snapshot() (raft.FSMSnapshot, error) {
	return &snapshot{cm: f.ctx.st.cm}, nil
}

// Restore stores the key-value store to a previous state.
// 根据快照恢复数据
// 服务重启的时候，会先读取本地的快照来恢复数据，
// 在FSM里面定义的Restore函数会被调用，这里我们就简单的对数据解析json反序列化然后写入内存即可
func (f *FSM) Restore(serialized io.ReadCloser) error {
	return f.ctx.st.cm.UnMarshal(serialized)
}

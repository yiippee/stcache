package main

import (
	"encoding/json"
	"io"
	"log"

	"github.com/hashicorp/raft"
)

/*

日志、FSM执行过程：
1，leader收到命令，立刻将命令写入本地日志中；
2，向其他follower发送该日志条目；
3，收到半数以上的follower响应后，就将该日志标识为已提交，并将该日志发往本地的FSM执行，执行完毕后返回结果给客户端；
4，只要日志写入了本地，则如果半数没有响应，或者说本地执行FSM失败（其实不可能失败），则只要恢复，则还是会继续执行 runFSM，一直在跑，直到commited
       有个 lastApplied 来记录着最后已经提交的日志id，那么所有大于这个编号的则视为未提交，会重新提交。编号是永远自增的
4，通过后续的通知follower哪些日志条目已经提交，以便follower在自己的fsm中执行日志。

*/

/*
The log stores are indeed unrelated to the FSM.
The FSM only applies committed entries, the store persists also entries that haven't been committed yet (because they are currently reaching a quorum).
The FSM should typically do only in-memory operations, yes.  FSM只管内存操作，与log存储毫无关系。
As said, at startup hashicorp's implementation will use the latest available snapshot, to reduce the overhead of rebuilding the FSM .
*/
/*
raft本身也维护了一个状态，包括当前的任期term，已经提交的最大的日志index，最后应用于FSM的日志index，最后执行快照的index和term，
最后的日志index和term。 定义在raftState中。这些信息是非常重要的，明确描述了当前状态。
*/

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

// 节点重启以后，加载快照数据也会调用这个函数来更新应用信息
func (f *FSM) Apply(logEntry *raft.Log) interface{} {
	e := logEntryData{}
	if err := json.Unmarshal(logEntry.Data, &e); err != nil {
		// 应用一条日志是不能有任何错误的，唯一的可能是内存不足，这个时候应该panic，日志存储不应该与FSM应用程序逻辑有任何关系
		panic("Failed unmarshaling Raft log entry. This is a bug.")
	}
	ret := f.ctx.st.cm.Set(e.Key, e.Value)
	f.log.Printf("fms.Apply(), logEntry:%s, ret:%v\n", logEntry.Data, ret)
	return ret
}

// Snapshot returns a latest snapshot
// 生成一个快照结构。快照的作用就是对已有的数据进行备份一次，那么之前的所有日志都可以删除了
func (f *FSM) Snapshot() (raft.FSMSnapshot, error) {
	return &snapshot{cm: f.ctx.st.cm}, nil
}

// Restore stores the key-value store to a previous state.
// 根据快照恢复数据
// 服务重启的时候，会先读取本地的快照来恢复数据，
// 在FSM里面定义的Restore函数会被调用，这里我们就简单的对数据解析json反序列化然后写入内存即可。  folloer重启的时候也是先读快照，再加上leader发送过来的最新的log
func (f *FSM) Restore(serialized io.ReadCloser) error {
	return f.ctx.st.cm.UnMarshal(serialized)
}

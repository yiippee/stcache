package main

import (
	"github.com/hashicorp/raft"
)

// snapshot代表了应用的状态数据，而执行snapshot的动作也就是将应用状态数据持久化存储，
// 这样，在该snapshot之前的所有日志便成为无效数据，可以删除。

// snapshot本质上是应用状态的一份拷贝，snapshot就是对内存当前状态进行照相保存在磁盘上。
// snapshot的主要目的是回收日志文件，
// 随着系统运行，raft使用的更新日志文件会越来越大，使用snapshot，对某时刻系统照相后，
// 那么当前系统的状态便会被永久记录，则此刻之前的更新日志便可被回收了。
// 如果接下来系统重启，只需要将该时刻的snapshot加载并重放该snapshot以后的更新日志即可重构系统奔溃之前的状态。
type snapshot struct {
	cm *cacheManager
}

// Persist saves the FSM snapshot out to the given sink.
// 需要实现两个func，Persist用来生成快照数据，一般只需要实现它即可；
// Release则是快照处理完成后的回调，不需要的话可以实现为空函数。
func (s *snapshot) Persist(sink raft.SnapshotSink) error {
	snapshotBytes, err := s.cm.Marshal()
	if err != nil {
		sink.Cancel()
		return err
	}

	if _, err := sink.Write(snapshotBytes); err != nil {
		sink.Cancel()
		return err
	}

	if err := sink.Close(); err != nil {
		sink.Cancel()
		return err
	}
	return nil
}

func (f *snapshot) Release() {}

package main

import (
	"github.com/hashicorp/raft"
)

// snapshot代表了应用的状态数据，而执行snapshot的动作也就是将应用状态数据持久化存储，
// 这样，在该snapshot之前的所有日志便成为无效数据，可以删除。
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

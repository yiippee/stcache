package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb"
)

type raftNodeInfo struct {
	raft           *raft.Raft
	fsm            *FSM
	leaderNotifyCh chan bool
}

func newRaftTransport(opts *options) (*raft.NetworkTransport, error) {
	address, err := net.ResolveTCPAddr("tcp", opts.raftTCPAddress)
	if err != nil {
		return nil, err
	}

	// 采用hashicorp/raft内部提供的TCPTransport来作为集群节点之间的日志同步、leader选举等通信
	transport, err := raft.NewTCPTransport(address.String(), address, 3, 10*time.Second, os.Stderr)
	if err != nil {
		return nil, err
	}
	return transport, nil
}

func newRaftNode(opts *options, ctx *stCachedContext) (*raftNodeInfo, error) {
	raftConfig := raft.DefaultConfig()                      // 直接使用raft默认的配置
	raftConfig.LocalID = raft.ServerID(opts.raftTCPAddress) // 用监听的地址来作为节点的id
	// raftConfig.Logger = hclog.New(os.Stderr, "raft: ", log.Ldate|log.Ltime)
	// 因为snapshot创建是有代价的，因此，这个频率不能太高，在示例应用中，每更新10000条日志才会进行一次snapshot创建。
	raftConfig.SnapshotInterval = 10 * time.Second // 每间隔多久生成一次快照,这里是20s
	// 因为snapshot创建是有代价的，因此，这个频率不能太高，在示例应用中，每更新10000条日志才会进行一次snapshot创建。
	raftConfig.SnapshotThreshold = 5 // 每commit多少log entry后生成一次快照,这里是2条
	raftConfig.TrailingLogs = 0      // 执行一次快照之后，需要保留多少日志。 这样我们就可以快速地在追随者上重放日志，而不是被迫发送整个快照。

	leaderNotifyCh := make(chan bool, 1)
	raftConfig.NotifyCh = leaderNotifyCh

	transport, err := newRaftTransport(opts)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(opts.dataDir, 0700); err != nil {
		return nil, err
	}

	fsm := &FSM{
		ctx: ctx,
		log: log.New(os.Stderr, "FSM: ", log.Ldate|log.Ltime),
	}
	// 直接使用hashicorp提供的raft-boltdb来实现底层存储
	snapshotStore, err := raft.NewFileSnapshotStore(opts.dataDir, 1 /*需要保留多少分快照*/, os.Stderr)
	if err != nil {
		return nil, err
	}

	logStore, err := raftboltdb.NewBoltStore(filepath.Join(opts.dataDir, "raft-log.bolt"))
	if err != nil {
		return nil, err
	}

	/*
		hashicorp内部提供3中快照存储方式，分别是：

		DiscardSnapshotStore：  不存储，忽略快照，相当于/dev/null，一般用于测试
		FileSnapshotStore：        文件持久化存储
		InmemSnapshotStore：   内存存储，不持久化，重启程序会丢失

		这里我们使用文件持久化存储。snapshotStore只是提供了一个快照存储的介质
	*/
	stableStore, err := raftboltdb.NewBoltStore(filepath.Join(opts.dataDir, "raft-stable.bolt"))
	if err != nil {
		return nil, err
	}

	raftNode, err := raft.NewRaft(raftConfig /*节点配置*/, fsm, /*有限状态机*/
		logStore /*raft日志存储*/, stableStore, /*稳定存储，用来存储raft集群的节点信息等*/
		snapshotStore /*快照存储，用来存储节点的快照信息，就是存储当前的所有的kv数据，就是相当于数据库存储*/, transport /*raft节点内部的通信通道*/)

	if err != nil {
		return nil, err
	}

	//  集群最开始的时候只有一个节点，我们让第一个节点通过bootstrap的方式启动，
	//  它启动后成为leader。
	if opts.bootstrap {
		configuration := raft.Configuration{
			Servers: []raft.Server{
				{
					ID:      raftConfig.LocalID,
					Address: transport.LocalAddr(),
				},
			},
		}
		raftNode.BootstrapCluster(configuration)
	}

	return &raftNodeInfo{raft: raftNode, fsm: fsm, leaderNotifyCh: leaderNotifyCh}, nil
}

// joinRaftCluster joins a node to raft cluster
func joinRaftCluster(opts *options) error {
	url := fmt.Sprintf("http://%s/join?peerAddress=%s", opts.joinAddress, opts.raftTCPAddress)

	resp, err := http.Get(url)
	if err != nil {
		return err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if string(body) != "ok" {
		return errors.New(fmt.Sprintf("Error joining cluster: %s", body))
	}

	return nil
}

type Cluster struct {
}

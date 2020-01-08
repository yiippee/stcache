package main

import (
	"flag"
)

type options struct {
	dataDir        string // data directory
	httpAddress    string // http server address
	raftTCPAddress string // construct Raft Address
	bootstrap      bool   // start as master or not
	joinAddress    string // peer address to join
}

func NewOptions() *options {
	opts := &options{}

	// node1
	//// 本节点监听的http地址
	//var httpAddress = flag.String("http", ":6000", "Http address")
	//// 本节点用于raft之间通信的地址
	//var raftTCPAddress = flag.String("raft", "127.0.0.1:7000", "raft tcp address")
	//// var raftTCPAddress = flag.String("raft", "", "raft tcp address")
	//// 本节点的节点名字
	//var node = flag.String("node", "node1", "raft node name")
	//// 是否以leader节点启动
	//var bootstrap = flag.Bool("bootstrap", true, "start as raft cluster")
	////var joinAddress = flag.String("join", "127.0.0.1:6001", "join address for raft cluster")
	//// 本节点需要加入的远程节点（主节点？）的地址
	//var joinAddress = flag.String("join", "", "join address for raft cluster")

	// node 2
	// 本节点监听的http地址
	var httpAddress = flag.String("http", ":6003", "Http address")
	// 本节点用于raft之间通信的地址
	var raftTCPAddress = flag.String("raft", "127.0.0.1:7003", "raft tcp address")
	// var raftTCPAddress = flag.String("raft", "", "raft tcp address")
	// 本节点的节点名字
	var node = flag.String("node", "node4", "raft node name")
	// 是否以leader节点启动
	var bootstrap = flag.Bool("bootstrap", false, "start as raft cluster")
	//var joinAddress = flag.String("join", "127.0.0.1:6001", "join address for raft cluster")
	// 本节点需要加入的远程节点（主节点？）的地址
	var joinAddress = flag.String("join", "", "join address for raft cluster")

	flag.Parse()

	opts.dataDir = "./" + *node
	opts.httpAddress = *httpAddress
	opts.bootstrap = *bootstrap
	opts.raftTCPAddress = *raftTCPAddress
	opts.joinAddress = *joinAddress
	return opts
}

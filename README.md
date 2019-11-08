# stcache
A simple cache server showing how to use hashicorp/raft

使用run.sh启动
 主节点启动方式为 ./run.sh 1 1  第一个1说明是第一个节点，第二个 1 说明是master启动，第一次启动会报错，因为没有node1目录，后面就不会
 其他节点类似启动 ./run.sh 2  但每次启动都会申请加入主节点，这在重新启动时可能会报错，可以删除主节点log信息
                ./run.sh 3
                ./run.sh 4

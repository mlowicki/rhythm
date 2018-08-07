package main

import (
	"strconv"
	"strings"

	"github.com/samuel/go-zookeeper/zk"
)

func registerForElection(conn *zk.Conn) (int64, error) {
	// TODO Use CreateProtectedEphemeralSequential
	name, err := conn.Create("/rhythm/election/", []byte(""), zk.FlagEphemeral|zk.FlagSequence, zk.WorldACL(zk.PermAll))
	if err != nil {
		return 0, err
	}
	parts := strings.Split(name, "/")
	num, err := strconv.ParseInt(parts[len(parts)-1], 10, 64)
	return num, err
}

func getNumbers(conn *zk.Conn) ([]int64, <-chan zk.Event, error) {
	names, _, ch, err := conn.ChildrenW("/rhythm/election")
	if err != nil {
		return nil, nil, err
	}
	nums := make([]int64, len(names))
	for i, name := range names {
		num, err := strconv.ParseInt(name, 10, 64)
		if err != nil {
			return nil, nil, err
		}
		nums[i] = num
	}
	return nums, ch, nil
}

func isLeader(conn *zk.Conn, num int64) (bool, <-chan zk.Event, error) {
	ns, ch, err := getNumbers(conn)
	if err != nil {
		return false, nil, err
	}
	min := ns[0]
	for _, n := range ns {
		if n < min {
			min = n
		}
	}
	// TODO close this channel if leader?
	return num == min, ch, nil
}

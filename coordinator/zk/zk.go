package zk

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"

	"github.com/mlowicki/rhythm/conf"
	"github.com/samuel/go-zookeeper/zk"
)

type Coordinator struct {
	basePath    string
	electionDir string
	conn        *zk.Conn
	ticket      string
	eventChan   <-chan zk.Event
	cancel      context.CancelFunc
	sync.Mutex
}

func (coord *Coordinator) WaitUntilLeader() (context.Context, error) {
	isLeader, ch, err := coord.isLeader()
	if err != nil {
		return nil, err
	}
	if !isLeader {
		for {
			log.Println("coordinator: Not elected as leader. Waiting...")
			<-ch
			isLeader, ch, err = coord.isLeader()
			if err != nil {
				return nil, err
			} else if isLeader {
				break
			}
		}
	}
	log.Println("coordinator: Elected as leader")
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	coord.Lock()
	coord.cancel = cancel
	coord.Unlock()
	return ctx, nil
}

func (coord *Coordinator) register() error {
	// TODO Consider using `CreateProtectedEphemeralSequential`
	name, err := coord.conn.Create(coord.basePath+"/"+coord.electionDir+"/", []byte(""), zk.FlagEphemeral|zk.FlagSequence, zk.WorldACL(zk.PermAll))
	if err != nil {
		return err
	}
	parts := strings.Split(name, "/")
	coord.Lock()
	coord.ticket = parts[len(parts)-1]
	coord.Unlock()
	return nil
}

func (coord *Coordinator) isLeader() (bool, <-chan zk.Event, error) {
	coord.Lock()
	ticket := coord.ticket
	coord.Unlock()
	if ticket == "" {
		err := coord.register()
		if err != nil {
			return false, nil, fmt.Errorf("coordinator: Registration failed: %s\n", err)
		}
	}
	tickets, _, eventChan, err := coord.conn.ChildrenW(coord.basePath + "/" + coord.electionDir)
	if err != nil {
		return false, nil, fmt.Errorf("coordinator: Failed getting registration tickets: %s\n", err)
	}
	coord.Lock()
	ticket = coord.ticket
	coord.Unlock()
	isLeader := false
	sort.Strings(tickets)
	if len(tickets) > 0 {
		if tickets[0] == ticket {
			isLeader = true
		}
	}
	log.Printf("coordinator: All registration tickets: %v\n", tickets)
	log.Printf("coordinator: My registration ticket: %s\n", ticket)
	for _, cur := range tickets {
		if ticket == cur {
			return isLeader, eventChan, nil
		}
	}
	return false, nil, fmt.Errorf("coordinator: Registration ticket doesn't exist")
}

func (coord *Coordinator) initZK() error {
	electionPath := coord.basePath + "/" + coord.electionDir
	exists, _, err := coord.conn.Exists(electionPath)
	if err != nil {
		return fmt.Errorf("coordinator: Failed checking if election directory exists: %s\n", err)
	}
	if !exists {
		_, err = coord.conn.Create(electionPath, []byte{}, 0, zk.WorldACL(zk.PermAll))
		if err != nil {
			return fmt.Errorf("coordinator: Failed creating election directory: %s\n", err)
		}
	}
	return nil
}

func New(conf *conf.CoordinatorZooKeeper) (*Coordinator, error) {
	conn, eventChan, err := zk.Connect(conf.Servers, conf.Timeout)
	if err != nil {
		return nil, fmt.Errorf("coordinator: Failed connecting to ZooKeeper: %s\n", err)
	}
	coord := Coordinator{
		conn:        conn,
		basePath:    conf.BasePath,
		electionDir: conf.ElectionDir,
		eventChan:   eventChan,
	}
	err = coord.initZK()
	if err != nil {
		conn.Close()
		return nil, err
	}
	go func() {
		for {
			select {
			case ev := <-coord.eventChan:
				log.Printf("coordinator: ZooKeeper event: %s\n", ev)
				if ev.State == zk.StateDisconnected {
					log.Printf("coordinator: Disconnected from ZooKeeper: %s\n", ev)
					coord.Lock()
					if coord.cancel != nil {
						coord.cancel()
						coord.cancel = nil
					}
					coord.Unlock()
				} else if ev.State == zk.StateExpired {
					log.Printf("coordinator: Session expired: %s\n", ev)
					coord.Lock()
					coord.ticket = ""
					coord.Unlock()
				}
			}
		}
	}()
	return &coord, nil
}

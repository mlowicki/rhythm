package main

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

type election struct {
	basePath    string
	electionDir string
	conn        *zk.Conn
	ticket      string
	eventChan   <-chan zk.Event
	cancel      context.CancelFunc
	sync.Mutex
}

func (elec *election) WaitUntilLeader() (context.Context, error) {
	isLeader, ch, err := elec.isLeader()
	if err != nil {
		return nil, err
	}
	if !isLeader {
		for {
			log.Println("election: Not elected as leader. Waiting...")
			<-ch
			isLeader, ch, err = elec.isLeader()
			if err != nil {
				return nil, err
			} else if isLeader {
				break
			}
		}
	}
	log.Println("election: Elected as leader")
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	elec.Lock()
	elec.cancel = cancel
	elec.Unlock()
	return ctx, nil
}

func (elec *election) register() error {
	// TODO Consider using `CreateProtectedEphemeralSequential`
	name, err := elec.conn.Create(elec.basePath+"/"+elec.electionDir+"/", []byte(""), zk.FlagEphemeral|zk.FlagSequence, zk.WorldACL(zk.PermAll))
	if err != nil {
		return err
	}
	parts := strings.Split(name, "/")
	elec.Lock()
	elec.ticket = parts[len(parts)-1]
	elec.Unlock()
	return nil
}

func (elec *election) isLeader() (bool, <-chan zk.Event, error) {
	elec.Lock()
	ticket := elec.ticket
	elec.Unlock()
	if ticket == "" {
		err := elec.register()
		if err != nil {
			return false, nil, fmt.Errorf("election: Registration failed: %s\n", err)
		}
	}
	tickets, _, eventChan, err := elec.conn.ChildrenW(elec.basePath + "/" + elec.electionDir)
	if err != nil {
		return false, nil, fmt.Errorf("election: Failed getting registration tickets: %s\n", err)
	}
	elec.Lock()
	ticket = elec.ticket
	elec.Unlock()
	isLeader := false
	sort.Strings(tickets)
	if len(tickets) > 0 {
		if tickets[0] == ticket {
			isLeader = true
		}
	}
	log.Printf("election: All registration tickets: %v\n", tickets)
	log.Printf("election: My registration ticket: %s\n", ticket)
	for _, cur := range tickets {
		if ticket == cur {
			return isLeader, eventChan, nil
		}
	}
	return false, nil, fmt.Errorf("election: Registration ticket doesn't exist")
}

func (elec *election) initZK() error {
	electionPath := elec.basePath + "/" + elec.electionDir
	exists, _, err := elec.conn.Exists(electionPath)
	if err != nil {
		return fmt.Errorf("election: Failed checking if election directory exists: %s\n", err)
	}
	if !exists {
		_, err = elec.conn.Create(electionPath, []byte{}, 0, zk.WorldACL(zk.PermAll))
		if err != nil {
			return fmt.Errorf("election: Failed creating election directory: %s\n", err)
		}
	}
	return nil
}

func newElection(conf *conf.ZooKeeper) (*election, error) {
	conn, eventChan, err := zk.Connect(conf.Servers, conf.Timeout)
	if err != nil {
		return nil, fmt.Errorf("election: Failed connecting to ZooKeeper: %s\n", err)
	}
	elec := election{
		conn:        conn,
		basePath:    conf.BasePath,
		electionDir: conf.ElectionDir,
		eventChan:   eventChan,
	}
	err = elec.initZK()
	if err != nil {
		conn.Close()
		return nil, err
	}
	go func() {
		for {
			select {
			case ev := <-elec.eventChan:
				log.Printf("election: ZooKeeper event: %s\n", ev)
				if ev.State == zk.StateDisconnected {
					log.Printf("election: Disconnected from ZooKeeper: %s\n", ev)
					elec.Lock()
					if elec.cancel != nil {
						elec.cancel()
						elec.cancel = nil
					}
					elec.Unlock()
				} else if ev.State == zk.StateExpired {
					log.Printf("election: Session expired: %s\n", ev)
					elec.Lock()
					elec.ticket = ""
					elec.Unlock()
				}
			}
		}
	}()
	return &elec, nil
}

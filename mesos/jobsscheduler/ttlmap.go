package jobsscheduler

import (
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

type ttlSet struct {
	m   map[string]time.Time
	mut sync.Mutex
	ttl time.Duration
}

func newTTLSet(ttl time.Duration) *ttlSet {
	s := &ttlSet{m: make(map[string]time.Time), ttl: ttl}
	go func() {
		for range time.Tick(time.Minute) {
			log.Debug("TTL set cleanup started")
			s.mut.Lock()
			for k, v := range s.m {
				if time.Now().Sub(v) > s.ttl {
					delete(s.m, k)
				}
			}
			s.mut.Unlock()
			log.Debug("TTL set cleanup finished")
		}
	}()
	return s
}

func (s *ttlSet) Exists(k string) bool {
	s.mut.Lock()
	v, ok := s.m[k]
	s.mut.Unlock()
	if !ok {
		return false
	}
	return time.Now().Sub(v) <= s.ttl
}

func (s *ttlSet) Set(k string) {
	s.mut.Lock()
	s.m[k] = time.Now()
	s.mut.Unlock()
}

func (s *ttlSet) Del(k string) {
	s.mut.Lock()
	delete(s.m, k)
	s.mut.Unlock()
}

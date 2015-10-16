// Copyright 2014 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package common

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

var errAcquireTimeout = errors.New("acquire timeout")

type cntSema struct {
	value, capacity uint32
	cond            *sync.Cond
}

func NewCntSema(capacity uint32) *cntSema {
	if capacity == 0 {
		// TODO: error
	}
	var m sync.Mutex
	cond := sync.NewCond(&m)
	return &cntSema{
		cond:     cond,
		capacity: capacity,
		value:    capacity,
	}
}

func (s *cntSema) get() uint32 {
	return atomic.LoadUint32(&s.value)
}

func (s *cntSema) Acquire(n uint32, timeout time.Duration) error {
	if n > s.capacity {
		return fmt.Errorf("requested amount %d exceeds semaphore capacity %d", n, s.capacity)
	}

	var timer *time.Timer
	var expired uint32

	for {
		s.cond.L.Lock()
		v := atomic.LoadUint32(&s.value)
		if v >= n {
			if atomic.CompareAndSwapUint32(&s.value, v, v-n) {
				s.cond.L.Unlock()
				return nil
			}
		}

		// Start the timeout on the first iteration.
		if timer == nil {
			timer = time.AfterFunc(timeout, func() {
				atomic.AddUint32(&expired, 1)
				// would be nice to wake up only this caller, but cond has no
				// API for this; cond.Signal() wakes up any of the routines
				s.cond.Broadcast()
			})
			defer timer.Stop()
		}

		s.cond.Wait()     // Wait until next release
		s.cond.L.Unlock() // always locked when Wait returns, but we have no need for the lock here

		if atomic.LoadUint32(&expired) == 1 {
			return errAcquireTimeout
		}
	}
}

func (s *cntSema) Release(n uint32) error {
	for {
		v := atomic.LoadUint32(&s.value)
		if v+n > s.capacity {
			return fmt.Errorf("semaphore count %d would exceed capacity %d after release of %d", v+n, s.capacity, n)
		}

		if atomic.CompareAndSwapUint32(&s.value, v, v+n) {
			s.cond.L.Lock()
			s.cond.Broadcast()
			s.cond.L.Unlock()
			return nil
		}
	}
}

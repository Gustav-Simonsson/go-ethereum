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
	crand "crypto/rand"
	"errors"
	"fmt"
	"math"
	"math/big"
	mrand "math/rand"
	"reflect"
	"sync"
	"testing"
	"testing/quick"
	"time"
)

func TestCntSemaCountSimple(t *testing.T) {
	sem := NewCntSema(2000)

	checkacquire := func(count, wantCount uint32, wantErr error) {
		err := sem.Acquire(count, 10*time.Millisecond)
		if !reflect.DeepEqual(err, wantErr) {
			t.Fatalf("wrong error after acquire(%d): got %q, want %q", count, err, wantErr)
		}
		if val := sem.get(); val != wantCount {
			t.Fatalf("wrong count after acquire(%d): got %d, want %d", count, val, wantCount)
		}
	}
	checkRelease := func(count, wantCount uint32) {
		sem.Release(count)
		if val := sem.get(); val != wantCount {
			t.Fatalf("wrong count after Release(%d): got %d, want %d", count, val, wantCount)
		}
	}

	// Check that the counter is maintained correctly.
	checkacquire(1000, 1000, nil)
	checkacquire(1000, 0, nil)
	checkacquire(1000, 0, errAcquireTimeout)
	checkRelease(900, 900)
	checkRelease(900, 1800)
	checkRelease(199, 1999)
	checkRelease(1, 2000)

	// Check that requesting more than sem.cap fails.
	checkacquire(2001, 2000, errors.New("requested amount 2001 exceeds semaphore capacity 2000"))

	// Check that a failed Acquire leaves sem.val as is when it is < sem.cap.
	checkacquire(500, 1500, nil)
	checkRelease(200, 1700)
	checkacquire(2000, 1700, errAcquireTimeout)
}

// This test checks that Release wakes up Acquire.
func TestCntSemaRace(t *testing.T) {
	const (
		waitCount  = 10000
		iterations = 15000
	)
	sem := NewCntSema(waitCount)
	pleaseRelease := make(chan uint32, 500)

	w := new(sync.WaitGroup)

	Releaser := func() {
		for rv := range pleaseRelease {
			sem.Release(rv)
		}
		w.Done()
	}

	defer func() {
		close(pleaseRelease)
		w.Wait()
		c := sem.get()
		if c != waitCount {
			t.Fatalf("unexpected final waitcount: %d", c)
		}
	}()

	w.Add(3)

	go Releaser()
	go Releaser()
	go Releaser()

	for i := 0; i < iterations; i++ {
		if err := sem.Acquire(waitCount, 1*time.Second); err != nil {
			t.Fatalf("iteration %d: %v (count: %d)", i, err, sem.get())
		}
		for i := uint32(0); i < waitCount; {
			rv := mrand.Uint32() % waitCount
			if i+rv > waitCount {
				rv = waitCount - i
			}
			i += rv
			pleaseRelease <- rv
		}
	}
}

// property-based test
type cntSemaTest struct {
	capacity, acquirers, releasers uint32
	acquireTimeout                 time.Duration
	goSched                        bool
}

func TestCntSemaQuick(t *testing.T) {
	seed, _ := crand.Int(crand.Reader, big.NewInt(math.MaxInt64))
	fmt.Println("quick testing using seed:", seed.Int64())
	testRand := mrand.New(mrand.NewSource(seed.Int64()))

	f := func(cst cntSemaTest) {

	}

	config := &quick.Config{Rand: testRand}
	if err := quick.Check(f, config); err != nil {
		t.Error(err)
	}
}

func (cst cntSemaTest) Generate(rand *mrand.Rand, size int) reflect.Value {
	st := cntSemaTest{
		capacity:       uint32(rand.Int()),
		acquirers:      uint32(rand.Int()) % 20,
		releasers:      uint32(rand.Int()) % 20,
		acquireTimeout: (time.Duration(rand.Int()%100) * 1000000), // 0-100 ms
		goSched:        (rand.Int() % 2) == 0,
	}
	return reflect.ValueOf(st)
}

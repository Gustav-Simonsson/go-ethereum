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

package state

import (
	"bytes"
	"fmt"
	"io"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/logger"
	"github.com/ethereum/go-ethereum/logger/glog"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"
)

var emptyCodeHash = crypto.Keccak256(nil)

type Code []byte

func (self Code) String() string {
	return string(self) //strings.Join(Disassemble(self), " ")
}

type Storage map[common.Hash]common.Hash

func (self Storage) String() (str string) {
	for key, value := range self {
		str += fmt.Sprintf("%X : %X\n", key, value)
	}

	return
}

func (self Storage) Copy() Storage {
	cpy := make(Storage)
	for key, value := range self {
		cpy[key] = value
	}

	return cpy
}

// StateObject represents an Ethereum account which is being modified.
//
// The usage pattern is as follows:
// First you need to obtain a state object.
// Account values can be accessed and modified through the object.
// Finally, call CommitTrie to write the modified storage trie into a database.
type StateObject struct {
	address common.Address // Ethereum address of this account
	data    Account

	// DB error.
	// State objects are used by the consensus core and VM which are
	// unable to deal with database-level errors. Any error that occurs
	// during a database read is memoized here and will eventually be returned
	// by StateDB.Commit.
	dbErr error

	// Write caches.
	trie    *trie.SecureTrie // storage trie, which becomes non-nil on first access
	code    Code             // contract bytecode, which gets set when code is loaded
	storage Storage          // Cached storage (flushed when updated)

	// Cache flags.
	// When an object is marked for deletion it will be delete from the trie
	// during the "update" phase of the state transition
	dirty     bool // true if anything has changed
	dirtyCode bool // true if the code was updated
	remove    bool
	deleted   bool
}

// Account is the Ethereum consensus representation of accounts.
// These objects are stored in the main account trie.
type Account struct {
	Nonce    uint64
	Balance  *big.Int
	Root     common.Hash // merkle root of the storage trie
	CodeHash []byte

	codeSize *int
}

type JournalEntry struct {
	Acc     Account
	AccAddr common.Address
	Root    common.Hash
}

func CopyAccount(acc Account) Account {
	newAcc := Account{
		Nonce:    acc.Nonce,
		CodeHash: acc.CodeHash,
		Root:     acc.Root,
		codeSize: new(int),
	}
	if acc.Balance != nil {
		newAcc.Balance = new(big.Int).Set(acc.Balance)
	}

	if acc.codeSize != nil {
		*newAcc.codeSize = *acc.codeSize
	}
	return newAcc
}

func NewJournalEntry(acc Account, addr common.Address, root common.Hash) JournalEntry {
	return JournalEntry{Acc: CopyAccount(acc), AccAddr: addr, Root: root}
}

// NewObject creates a state object.
func NewObject(address common.Address, data Account) *StateObject {
	if data.Balance == nil {
		data.Balance = new(big.Int)
	}
	if data.CodeHash == nil {
		data.CodeHash = emptyCodeHash
	}
	return &StateObject{address: address, data: data, storage: make(Storage)}
}

// EncodeRLP implements rlp.Encoder.
func (c *StateObject) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, c.data)
}

// setError remembers the first non-nil error it is called with.
func (self *StateObject) setError(err error) {
	if self.dbErr == nil {
		self.dbErr = err
	}
}

func (self *StateObject) MarkForDeletion() {
	self.remove = true
	self.dirty = true

	if glog.V(logger.Core) {
		glog.Infof("%x: #%d %v X\n", self.Address(), self.Nonce(), self.Balance())
	}
}

func (c *StateObject) initTrie(db trie.Database) {
	if c.trie == nil {
		var err error
		c.trie, err = trie.NewSecure(c.data.Root, db)
		if err != nil {
			c.trie, _ = trie.NewSecure(common.Hash{}, db)
			c.setError(fmt.Errorf("can't create storage trie: %v", err))
		}
	}
}

// GetState returns a value in account storage.
func (self *StateObject) GetState(db trie.Database, key common.Hash) common.Hash {
	value, exists := self.storage[key]
	if exists {
		return value
	}
	// Load from DB in case it is missing.
	self.initTrie(db)
	var ret []byte
	rlp.DecodeBytes(self.trie.Get(key[:]), &ret)
	value = common.BytesToHash(ret)
	if (value != common.Hash{}) {
		self.storage[key] = value
	}
	return value
}

// SetState updates a value in account storage.
func (self *StateObject) SetState(key, value common.Hash) {
	self.storage[key] = value
	self.dirty = true
}

// updateTrie writes cached storage modifications into the object's storage trie.
func (self *StateObject) updateTrie(db trie.Database) {
	self.initTrie(db)
	for key, value := range self.storage {
		if (value == common.Hash{}) {
			self.trie.Delete(key[:])
			continue
		}
		// Encoding []byte cannot fail, ok to ignore the error.
		v, _ := rlp.EncodeToBytes(bytes.TrimLeft(value[:], "\x00"))
		self.trie.Update(key[:], v)
	}
}

// UpdateRoot sets the trie root to the current root hash of
func (self *StateObject) UpdateRoot(db trie.Database) {
	self.updateTrie(db)
	self.data.Root = self.trie.Hash()
}

// CommitTrie the storage trie of the object to dwb.
// This updates the trie root.
func (self *StateObject) CommitTrie(db trie.Database, dbw trie.DatabaseWriter) error {
	self.updateTrie(db)
	if self.dbErr != nil {
		fmt.Println("dbErr:", self.dbErr)
		return self.dbErr
	}
	root, err := self.trie.CommitTo(dbw)
	if err == nil {
		self.data.Root = root
	}
	return err
}

func (c *StateObject) AddBalance(amount *big.Int) {
	c.SetBalance(new(big.Int).Add(c.Balance(), amount))

	if glog.V(logger.Core) {
		glog.Infof("%x: #%d %v (+ %v)\n", c.Address(), c.Nonce(), c.Balance(), amount)
	}
}

func (c *StateObject) SubBalance(amount *big.Int) {
	c.SetBalance(new(big.Int).Sub(c.Balance(), amount))

	if glog.V(logger.Core) {
		glog.Infof("%x: #%d %v (- %v)\n", c.Address(), c.Nonce(), c.Balance(), amount)
	}
}

func (c *StateObject) SetBalance(amount *big.Int) {
	c.data.Balance = amount
	c.dirty = true
}

// Return the gas back to the origin. Used by the Virtual machine or Closures
func (c *StateObject) ReturnGas(gas, price *big.Int) {}

func (self *StateObject) Copy(db trie.Database) *StateObject {
	stateObject := NewObject(self.address, self.data)
	stateObject.data.Balance.Set(self.data.Balance)
	stateObject.trie = self.trie
	stateObject.code = self.code
	stateObject.storage = self.storage.Copy()
	stateObject.remove = self.remove
	stateObject.dirty = self.dirty
	stateObject.dirtyCode = self.dirtyCode
	stateObject.deleted = self.deleted
	return stateObject
}

//
// Attribute accessors
//

// Returns the address of the contract/account
func (c *StateObject) Address() common.Address {
	return c.address
}

// Code returns the contract code associated with this object, if any.
func (self *StateObject) Code(db trie.Database) []byte {
	if self.code != nil {
		return self.code
	}
	if bytes.Equal(self.CodeHash(), emptyCodeHash) {
		return nil
	}
	code, err := db.Get(self.CodeHash())
	if err != nil {
		self.setError(fmt.Errorf("can't load code hash %x: %v", self.CodeHash(), err))
	}
	self.code = code
	return code
}

// CodeSize returns the size of the contract code associated with this object.
func (self *StateObject) CodeSize(db trie.Database) int {
	if self.data.codeSize == nil {
		self.data.codeSize = new(int)
		*self.data.codeSize = len(self.Code(db))
	}
	return *self.data.codeSize
}

func (self *StateObject) SetCode(code []byte) {
	self.code = code
	self.data.CodeHash = crypto.Keccak256(code)
	self.data.codeSize = new(int)
	*self.data.codeSize = len(code)
	self.dirty, self.dirtyCode = true, true
}

func (self *StateObject) SetNonce(nonce uint64) {
	self.data.Nonce = nonce
	self.dirty = true
}

func (self *StateObject) CodeHash() []byte {
	return self.data.CodeHash
}

func (self *StateObject) Balance() *big.Int {
	return self.data.Balance
}

func (self *StateObject) Nonce() uint64 {
	return self.data.Nonce
}

// Never called, but must be present to allow StateObject to be used
// as a vm.Account interface that also satisfies the vm.ContractRef
// interface. Interfaces are awesome.
func (self *StateObject) Value() *big.Int {
	panic("Value on StateObject should never be called")
}

func (self *StateObject) ForEachStorage(cb func(key, value common.Hash) bool) {
	// When iterating over the storage check the cache first
	for h, value := range self.storage {
		cb(h, value)
	}

	it := self.trie.Iterator()
	for it.Next() {
		// ignore cached values
		key := common.BytesToHash(self.trie.GetKey(it.Key))
		if _, ok := self.storage[key]; !ok {
			cb(key, common.BytesToHash(it.Value))
		}
	}
}

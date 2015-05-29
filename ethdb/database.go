package ethdb

import (
	"strings"

	"github.com/ethereum/go-ethereum/compression/rle"
	"github.com/ethereum/go-ethereum/logger"
	"github.com/ethereum/go-ethereum/logger/glog"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/errors"
	"github.com/syndtr/goleveldb/leveldb/iterator"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/storage"
)

var OpenFileLimit = 64

type DB struct {
	// filename for reporting
	fn string
	db *leveldb.DB
}

func OnDisk(file string) (*DB, error) {
	db, err := leveldb.OpenFile(file, &opt.Options{OpenFilesCacheCapacity: OpenFileLimit})
	// check for curruption and attempt to recover
	if _, iscorrupted := err.(*errors.ErrCorrupted); iscorrupted {
		db, err = leveldb.RecoverFile(file, nil)
	}
	// (re) check for errors and abort if opening of the db failed
	if err != nil {
		return nil, err
	}
	database := &DB{
		fn: file,
		db: db,
	}
	return database, nil
}

func InMemory(labels ...string) (*DB, error) {
	db, err := leveldb.Open(storage.NewMemStorage(), nil)
	if err != nil {
		return nil, err
	}
	return &DB{fn: strings.Join(labels, " "), db: db}, nil
}

func (self *DB) Put(key []byte, value []byte) {
	self.db.Put(key, rle.Compress(value), nil)
}

func (self *DB) Get(key []byte) ([]byte, error) {
	dat, err := self.db.Get(key, nil)
	if err != nil {
		return nil, err
	}
	return rle.Decompress(dat)
}

func (self *DB) Delete(key []byte) error {
	return self.db.Delete(key, nil)
}

func (self *DB) NewIterator() iterator.Iterator {
	return self.db.NewIterator(nil, nil)
}

func (self *DB) Close() {
	self.db.Close()
	glog.V(logger.Error).Infoln("Closed db:", self.fn)
}

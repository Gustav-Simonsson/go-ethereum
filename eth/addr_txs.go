// Copyright 2015 The go-ethereum Authors
// This file is part of go-ethereum.
//
// go-ethereum is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// go-ethereum is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with go-ethereum. If not, see <http://www.gnu.org/licenses/>.

package eth

import (
	"database/sql"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/logger"
	"github.com/ethereum/go-ethereum/logger/glog"
	_ "github.com/mattn/go-sqlite3"
)

const sqliteDBName = "addr_tx.db"

type AddrTxSyncer struct {
	txDB         *sql.DB
	chainDB      ethdb.Database
	bc           *core.BlockChain
	shutdownChan chan bool
	wg           sync.WaitGroup
}

func NewAddrTxSyncer(datadir string, chainDB ethdb.Database, bc *core.BlockChain) (*AddrTxSyncer, error) {
	s := AddrTxSyncer{}
	s.chainDB = chainDB
	s.bc = bc
	s.shutdownChan = make(chan bool)

	db, err := sql.Open("sqlite3", datadir+"/"+sqliteDBName)
	if err != nil {
		return nil, err
	}

	createStmt := `
	CREATE TABLE IF NOT EXISTS txs (hash CHARACTER(40) PRIMARY KEY NOT NULL, blocknumber INT4, sender CHARACTER(40), recipient CHARACTER(40));

	CREATE INDEX IF NOT EXISTS from_index ON txs (sender);
	CREATE INDEX IF NOT EXISTS to_index ON txs (recipient);
`

	_, err = db.Exec(createStmt)
	if err != nil {
		glog.V(logger.Error).Infof("Could not create sqlite db: %v", err)
		return nil, err
	}
	s.txDB = db
	// TODO: try out multiple sync threads and benchmark performance
	return &s, nil
}

func (s *AddrTxSyncer) Stop() {
	close(s.shutdownChan)
	s.wg.Wait()
	glog.V(logger.Info).Infof("Address Txs Syncer Stopped.\n")
}

func (s *AddrTxSyncer) ListTransactions(addr common.Address) ([]common.Hash, error) {
	addrStr := common.Bytes2Hex(addr[:])
	t0 := time.Now()
	sqlStmt := fmt.Sprintf("SELECT hash FROM txs WHERE sender = '%s' OR recipient = '%s'", addrStr)
	rows, err := s.txDB.Query(sqlStmt)
	fmt.Printf("FUNKY: select: %v\n", time.Since(t0).String())
	t0 = time.Now()
	if err != nil {
		return nil, err
	}

	var txHashes []common.Hash
	for rows.Next() {
		var txHash string
		rows.Scan(&txHash)
		//fmt.Printf("FUNKY: txHash: %v\n", txHash)
		txHashes = append(txHashes, common.HexToHash(txHash))
	}
	rows.Close()
	fmt.Printf("FUNKY: rows proc: %v\n", time.Since(t0).String())
	//fmt.Printf("FUNKY: txHashes: %v\n", txHashes)
	return txHashes, nil
}

func (s *AddrTxSyncer) SyncAddrTxs() error {
	s.wg.Add(1)
	defer s.wg.Done()

	rows, err := s.txDB.Query("SELECT blocknumber FROM txs ORDER BY blocknumber LIMIT 1")
	if err != nil {
		return err
	}
	lastBlockNum := uint64(0)
	if rows.Next() {
		rows.Scan(&lastBlockNum)
	}
	rows.Close()

	var headNumber *big.Int
	var blockHash common.Hash
	var block *types.Block
	var bn *big.Int

	blockHash = core.GetHeadBlockHash(s.chainDB)
	headBlock := core.GetBlock(s.chainDB, blockHash)
	headNumber = headBlock.Number()

	if lastBlockNum == 0 {
		block = headBlock
		bn = headNumber
	} else {
		block = s.bc.GetBlockByNumber(lastBlockNum)
		bn = new(big.Int).SetUint64(lastBlockNum)
	}
	glog.V(logger.Info).Infof("Loading addr_txs db, starting backwards traversal at block %v\n", bn)

	t0 := time.Now()

	progress := func() {
		t0 = time.Now()
		hnf := float64(headNumber.Uint64())
		bnf := float64(bn.Uint64())
		p := ((hnf - bnf) * 100) / hnf
		glog.V(logger.Info).Infof("Loading addr_txs db... %.3f%c\n", p, '%')
	}

	progress()

	for {
		select {
		case <-s.shutdownChan:
			return nil
		default:
		}
		if block == nil || bn.Cmp(common.Big0) == 0 {
			glog.V(logger.Info).Infof("Loading addr_txs db... done.\n")
			return nil
		}
		for _, tx := range block.Transactions() {
			from, _ := tx.From() // already validated
			h := tx.Hash()
			err := insertTx(s.txDB, &h, bn, &from, tx.To())
			if err != nil {
				return err
			}
		}
		if headNumber == nil {
			headNumber = bn
		}

		//t0 := time.Now()
		blockHash = block.ParentHash()
		block = core.GetBlock(s.chainDB, blockHash)
		bn = block.Number()
		//fmt.Printf("FUNKY: GetBlock: %v\n", time.Since(t0).String())

		if time.Since(t0) > 10*time.Second {
			progress()
		}
	}
	return nil
}

func insertTx(db *sql.DB, hash *common.Hash, blockNumber *big.Int, from, to *common.Address) error {
	// no to addr in contract deployment txs
	toStr := "NULL"
	if to != nil {
		toStr = common.Bytes2Hex(to[:])
	}

	// primary key collisions are ignored, can happen if interrupting
	// sync - then all txs in the last block are re-inserted
	sqlStmt :=
		fmt.Sprintf("INSERT OR IGNORE INTO txs(hash, blocknumber, sender, recipient) VALUES('%s', '%v', '%s', '%s');",
			common.Bytes2Hex(hash[:]),
			blockNumber,
			common.Bytes2Hex(from[:]),
			toStr)
	//fmt.Printf("FUNKY: sqlStmt:\n%s\n", sqlStmt)

	//t0 := time.Now()
	_, err := db.Exec(sqlStmt)
	//fmt.Printf("FUNKY: INSERT: %v\n", time.Since(t0).String())
	if err != nil {
		glog.V(logger.Error).Infof("Could not insert tx into addr_txs db: %v", err)
		return err
	}
	return nil
}

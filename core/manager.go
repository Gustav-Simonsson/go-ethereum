package core

import (
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
)

// TODO move this to types?
type Backend interface {
	AccountManager() *accounts.Manager
	BlockProcessor() *BlockProcessor
	ChainManager() *ChainManager
	TxPool() *TxPool
	BlockDb() *ethdb.DB
	StateDb() *ethdb.DB
	EventMux() *event.TypeMux
}

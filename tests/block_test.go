package tests

import (
	"fmt"
	"math/big"
	"path/filepath"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/ethdb"
)

func TestNonContractTxs(t *testing.T) {
	ks := crypto.NewKeyStorePassphrase(filepath.Join(common.DefaultDataDir(), "keystore"))

	cfg := &eth.Config{
		DataDir:        common.DefaultDataDir(),
		Verbosity:      5,
		Etherbase:      "primary",
		AccountManager: accounts.NewManager(ks),
		NewDB:          func(path string) (common.Database, error) { return ethdb.NewMemDatabase() },
	}

	ethereum, err := eth.New(cfg)

	err = ethereum.Start()

	genesis := core.GenesisBlock(0, ethereum.StateDb())
	ethereum.ResetWithGenesisBlock(genesis)

	cm := ethereum.ChainManager()
	statedb := cm.State()

	key, _ := crypto.GenerateKey()
	addr := crypto.PubkeyToAddress(key.PublicKey)
	obj := statedb.CreateAccount(addr)
	obj.SetBalance(big.NewInt(999999999999999))
	obj.SetNonce(0)

	statedb.Update()
	statedb.Sync()

	var txs types.Transactions
	for i := 1; i < 101; i++ {
		tx := types.NewTransactionMessage(common.Address{}, big.NewInt(100), big.NewInt(100), big.NewInt(100), nil)
		tx.GasLimit = big.NewInt(100000)
		k, _ := crypto.GenerateKey()
		to := crypto.PubkeyToAddress(k.PublicKey)
		tx.Recipient = &to
		tx.SetNonce(uint64(i))
		tx.SignECDSA(key)
		txs = append(txs, tx)
	}

	b := types.NewBlock(genesis.Hash(), common.Address{}, common.Hash{}, big.NewInt(100), 0, nil)
	t1 := time.Now()
	b.SetTransactions(txs)
	t2 := time.Since(t1)
	fmt.Println("FUNKY: ", t2.String())

	//b.SetUncles(nil)
	//b.SetRoot(statedb.Root())

	i, err := cm.InsertChain(types.Blocks{b})
	fmt.Println("FUNKY: ", i, err)
}

func TestBcValidBlockTests(t *testing.T) {
	err := RunBlockTest(filepath.Join(blockTestDir, "bcValidBlockTest.json"), BlockSkipTests)
	if err != nil {
		t.Fatal(err)
	}
}

func TestBcUncleTests(t *testing.T) {
	err := RunBlockTest(filepath.Join(blockTestDir, "bcUncleTest.json"), BlockSkipTests)
	if err != nil {
		t.Fatal(err)
	}
	err = RunBlockTest(filepath.Join(blockTestDir, "bcBruncleTest.json"), BlockSkipTests)
	if err != nil {
		t.Fatal(err)
	}
}

func TestBcUncleHeaderValidityTests(t *testing.T) {
	err := RunBlockTest(filepath.Join(blockTestDir, "bcUncleHeaderValiditiy.json"), BlockSkipTests)
	if err != nil {
		t.Fatal(err)
	}
}

func TestBcInvalidHeaderTests(t *testing.T) {
	err := RunBlockTest(filepath.Join(blockTestDir, "bcInvalidHeaderTest.json"), BlockSkipTests)
	if err != nil {
		t.Fatal(err)
	}
}

func TestBcInvalidRLPTests(t *testing.T) {
	err := RunBlockTest(filepath.Join(blockTestDir, "bcInvalidRLPTest.json"), BlockSkipTests)
	if err != nil {
		t.Fatal(err)
	}
}

func TestBcRPCAPITests(t *testing.T) {
	err := RunBlockTest(filepath.Join(blockTestDir, "bcRPC_API_Test.json"), BlockSkipTests)
	if err != nil {
		t.Fatal(err)
	}
}

func TestBcForkBlockTests(t *testing.T) {
	err := RunBlockTest(filepath.Join(blockTestDir, "bcForkBlockTest.json"), BlockSkipTests)
	if err != nil {
		t.Fatal(err)
	}
}

func TestBcTotalDifficulty(t *testing.T) {
	err := RunBlockTest(filepath.Join(blockTestDir, "bcTotalDifficultyTest.json"), BlockSkipTests)
	if err != nil {
		t.Fatal(err)
	}
}

func TestBcWallet(t *testing.T) {
	err := RunBlockTest(filepath.Join(blockTestDir, "bcWalletTest.json"), BlockSkipTests)
	if err != nil {
		t.Fatal(err)
	}
}

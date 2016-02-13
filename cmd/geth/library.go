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

// Contains a simple library definition to allow creating a Geth instance from
// straight C code.

package main

// #ifdef __cplusplus
// extern "C" {
// #endif
//
// extern int run(const char*);
//
// #ifdef __cplusplus
// }
// #endif
import "C"
import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

//export doRun
func doRun(args *C.char) C.int {
	// This is equivalent to geth.main, just modified to handle the function arg passing
	if err := app.Run(strings.Split("geth "+C.GoString(args), " ")); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return -1
	}
	return 0
}

//export doAccountNew
func doAccountNew(datadir, passphrase *C.char) C.int {
	// similar to 'geth account new'
	accman := lightAccountManager(C.GoString(datadir))
	_, err := accman.NewAccount(C.GoString(passphrase))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create account: %v\n", err)
		return -1
	}
	return 0
}

//export doUnlockAccount
func doUnlockAccount(datadir, addrStr, passphrase, timeoutStr *C.char) C.int {
	// similar to 'geth --unlock' + parseable timeoutStr
	timeout, err := time.ParseDuration(C.GoString(timeoutStr))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse duration string: %v\n", err)
		return -1
	}

	address := common.HexToAddress(C.GoString(addrStr))
	accman := lightAccountManager(C.GoString(datadir))

	err = accman.TimedUnlock(address, C.GoString(passphrase), timeout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to unlock account: %v\n", err)
		return -1
	}
	return 0
}

//export doLockAccount
func doLockAccount(datadir, addrStr *C.char) C.int {
	address := common.HexToAddress(C.GoString(addrStr))
	accman := lightAccountManager(C.GoString(datadir))
	err := accman.Lock(address)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to lock account: %v\n", err)
		return -1
	}
	return 0
}

// TODO: refactor dup with flags.MakeAccountManager
func lightAccountManager(datadir string) *accounts.Manager {
	// light KDF for android apps
	keystore := crypto.NewKeyStorePassphrase(
		filepath.Join(datadir, "keystore"),
		crypto.LightScryptN,
		crypto.LightScryptP)
	return accounts.NewManager(keystore)
}

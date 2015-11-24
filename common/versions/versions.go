// Copyright 2015 The go-ethereum Authors
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

package versions

import (
	"fmt"
	"strconv"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/logger"
	"github.com/ethereum/go-ethereum/logger/glog"
	"github.com/ethereum/go-ethereum/xeth"
)

var (
	// TODO: add Frontier address
	GlobalVersionsAddr   = "0x40bebcadbb4456db23fda39f261f3b2509096e9e" // test
	dummySender          = "0x16db48070243bc37a1c59cd5bb977ad7047618be" // test
	getVersionsSignature = "GetVersions()"

	jsonlogger = logger.NewJsonLogger()
)

// query versions list from the(custom) accessor in the versions contract
func Get(x *xeth.XEth, clientVersion string) (string, error) {
	// TODO: move common/registrar abiSignature to some util package
	abi := common.ToHex(crypto.Sha3([]byte(getVersionsSignature))[:4])
	res, _, err := x.Call(
		dummySender,
		GlobalVersionsAddr,
		"", "3000000", "",
		abi,
	)
	if err != nil {
		return "", err
	}

	// TODO: we use static arrays of size versionCount as workaround
	// until solidity has proper support for returning dynamic arrays
	versionCount := 10

	if len(res) != 2+(64*versionCount*3) { // 0x + three 32-byte fields per version
		return "", fmt.Errorf("unexpected result length from GetVersions")
	}

	// TODO: use ABI (after solidity supports returning arrays of arrays and/or structs)
	var versions []string
	var timestamps []uint64
	var signerCounts []uint64

	// trim 0x
	res = res[2:]

	// parse res
	for i := 0; i < versionCount; i++ {
		bytes := common.FromHex(res[:64])
		versions = append(versions, string(bytes))
		res = res[64:]
	}

	for i := 0; i < versionCount; i++ {
		ts, err := strconv.ParseUint(res[:64], 16, 64)
		if err != nil {
			return "", err
		}
		timestamps = append(timestamps, ts)
		res = res[64:]
	}

	for i := 0; i < versionCount; i++ {
		sc, err := strconv.ParseUint(res[:64], 16, 64)
		if err != nil {
			return "", err
		}
		signerCounts = append(signerCounts, sc)
		res = res[64:]
	}

	// TODO: version matching logic (e.g. most votes / most recent)
	if versions[0] != clientVersion {
		glog.V(logger.Info).Infof("geth version %s does not match recommended version %s", clientVersion, versions[0])
	}

	return res, nil
}

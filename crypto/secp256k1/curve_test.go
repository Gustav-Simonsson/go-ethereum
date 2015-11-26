package secp256k1

import (
	"bytes"
	"encoding/hex"
	"math/big"
	"testing"
)

func TestReadBits(t *testing.T) {
	check := func(input string) {
		want, _ := hex.DecodeString(input)
		int, _ := new(big.Int).SetString(input, 16)
		buf := make([]byte, len(want))
		readBits(buf, int)
		if !bytes.Equal(buf, want) {
			t.Errorf("have: %x\nwant: %x", buf, want)
		}
	}
	check("000000000000000000000000000000000000000000000000000000FEFCF3F8F0")
	check("0000000000012345000000000000000000000000000000000000FEFCF3F8F0")
	check("18F8F8F1000111000110011100222004330052300000000000000000FEFCF3F8F0")
}

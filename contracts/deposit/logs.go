package deposit

import (
	"bytes"
	"github.com/amazechain/amc/accounts/abi"
	"github.com/amazechain/amc/common/hexutil"
	"github.com/amazechain/amc/log"
	"github.com/holiman/uint256"
	"github.com/pkg/errors"
	"math/big"
)

// UnpackDepositLogData unpacks the data from a deposit log using the ABI decoder.
func UnpackDepositLogData(data []byte) (pubkey []byte, amount *uint256.Int, signature []byte, err error) {
	reader := bytes.NewReader(depositAbiCode)
	contractAbi, err := abi.JSON(reader)
	if err != nil {
		return nil, uint256.NewInt(0), nil, errors.Wrap(err, "unable to generate contract abi")
	}

	unpackedLogs, err := contractAbi.Unpack("DepositEvent", data)
	if err != nil {
		return nil, uint256.NewInt(0), nil, errors.Wrap(err, "unable to unpack logs")
	}
	amount, overflow := uint256.FromBig(unpackedLogs[1].(*big.Int))
	if overflow {
		return nil, uint256.NewInt(0), nil, errors.New("unable to unpack amount")
	}

	log.Debug("unpacked DepositEvent Logs", "publicKey", hexutil.Encode(unpackedLogs[0].([]byte)), "signature", hexutil.Encode(unpackedLogs[2].([]byte)), "message", hexutil.Encode(amount.Bytes()))

	return unpackedLogs[0].([]byte), amount, unpackedLogs[2].([]byte), nil
}

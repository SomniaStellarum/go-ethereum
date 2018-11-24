package ethclient

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
)

// Header represents a block header in the Ethereum blockchain.
// Local version to ethclient to allow non-required fields.
// Bloom and BlockNonce are []byte. Do json.Unmarshall on them when creating
// types.Header.
type Header struct {
	ParentHash  common.Hash    `json:"parentHash"       gencodec:"required"`
	UncleHash   common.Hash    `json:"sha3Uncles"       gencodec:"required"`
	Coinbase    common.Address `json:"miner"            gencodec:"required"`
	Root        common.Hash    `json:"stateRoot"        gencodec:"required"`
	TxHash      common.Hash    `json:"transactionsRoot" gencodec:"required"`
	ReceiptHash common.Hash    `json:"receiptsRoot"     gencodec:"required"`
	Bloom       []byte         `json:"logsBloom"        gencodec:"required"`
	Difficulty  *big.Int       `json:"difficulty"       gencodec:"required"`
	Number      *big.Int       `json:"number"           gencodec:"required"`
	GasLimit    uint64         `json:"gasLimit"         gencodec:"required"`
	GasUsed     uint64         `json:"gasUsed"          gencodec:"required"`
	Time        *big.Int       `json:"timestamp"        gencodec:"required"`
	Extra       []byte         `json:"extraData"        gencodec:"required"`
	MixDigest   common.Hash    `json:"mixHash"`
	Nonce       []byte         `json:"nonce"`
}

// field type overrides for gencodec
type headerMarshaling struct {
	Difficulty *hexutil.Big
	Number     *hexutil.Big
	GasLimit   hexutil.Uint64
	GasUsed    hexutil.Uint64
	Time       *hexutil.Big
	Extra      hexutil.Bytes
}

func (h *Header) convertHeader() (header *types.Header, err error) {
	header.ParentHash = h.ParentHash
	header.UncleHash = h.UncleHash
	header.Coinbase = h.Coinbase
	header.Root = h.Root
	header.TxHash = h.TxHash
	header.ReceiptHash = h.ReceiptHash
	if err := json.Unmarshal(h.Bloom, header.Bloom); err != nil {
		return nil, errors.New("Unmarshalling Bloom Error")
	}
	header.Difficulty = h.Difficulty
	header.Number = h.Number
	header.GasLimit = h.GasLimit
	header.GasUsed = h.GasUsed
	header.Time = h.Time
	header.Extra = h.Extra
	header.MixDigest = h.MixDigest
	fmt.Println(header.MixDigest)
	// if h.MixDigest == nil {
	// 	header.MixDigest = common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000000")
	// } else {
	// 	header.MixDigest = h.MixDigest
	// }
	if h.Nonce == nil {
		header.Nonce = types.EncodeNonce(0)
	} else {
		if err := json.Unmarshal(h.Nonce, header.Nonce); err != nil {
			return nil, errors.New("Unmarshalling Nonce Error")
		}
	}
	return header, nil
}

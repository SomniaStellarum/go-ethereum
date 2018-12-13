package kevm

import (
	"encoding/binary"
	"log"
	"math/big"
	"net"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/params"
	"github.com/golang/protobuf/proto"
)

type GetHash func(uint64) common.Hash

type Server struct {
	server  net.Conn
	stateDB StateDB
	getHash GetHash
}

type StateDB interface {
	GetBalance(common.Address) *big.Int
	GetNonce(common.Address) uint64
	GetCodeHash(common.Address) common.Hash
	GetCode(common.Address) []byte
	GetCodeSize(common.Address) int
	GetState(common.Address, common.Hash) common.Hash
}

func NewServer(chainConfig *params.ChainConfig, stateDB StateDB, getHash GetHash) (s *Server, err error) {
	s.server, err = net.Dial("tcp", "http://localhost:8080")
	if err != nil {
		return nil, err
	}
	s.stateDB = stateDB
	s.getHash = getHash
	ethConfig := &EthereumConfig{}
	// TODO: Determine what config needs to be set to.
	ethConfig.HomesteadBlockNumber, err = chainConfig.HomesteadBlock.MarshalText()
	if err != nil {
		// TODO: Need to review how error handling in this function works
		log.Panic("Homestead Block not marshalled: ", err)
	}
	hello_ethConfig := &Hello_EthereumConfig{EthereumConfig: ethConfig}
	hello := &Hello{
		Config: hello_ethConfig,
	}
	b, err := proto.Marshal(hello)
	if err != nil {
		log.Panic("Marshal Error of Hello Message: ", err)
	}
	_, err = s.server.Write(b)
	if err != nil {
		log.Panic("Error sending Hello Message: ", err)
	}
	return s, err
}

func (s *Server) Call(caller common.Address, addr common.Address, input []byte, gas uint64, value *big.Int) (ret []byte, leftOverGas uint64, err error) {
	// TODO: Send CallContext Message
	callCtx := &CallContext{}
	callCtx.CallerAddr = caller.Bytes()
	callCtx.RecipientAddr = addr.Bytes()
	callCtx.InputData = input
	callCtx.CallValue = value.Bytes()
	binary.BigEndian.PutUint64(callCtx.GasProvided, gas)
	result, err := s.sendAndReturn(callCtx)
	if err != nil {
		return nil, 0, err
	}
	// TODO: Error Check and do we need to change Modified/Deleted accounts?
	ret = result.ReturnData
	leftOverGas = binary.BigEndian.Uint64(result.GasRemaining)
	return ret, leftOverGas, nil
}

func (s *Server) Create(caller common.Address, code []byte, gas uint64, value *big.Int) (ret []byte, contractAddr common.Address, leftOverGas uint64, err error) {
	// TODO: Send CallContext Message
	// Similar to Call above, but also need to return created Contract Address
	// This info likely (but need to check) in ModifiedAccount
	return ret, contractAddr, leftOverGas, nil
}

func (s *Server) sendAndReturn(ctx *CallContext) (result *CallResult, err error) {
	b, err := proto.Marshal(ctx)
	if err != nil {
		return nil, err
	}
	_, err = s.server.Write(b)
	if err != nil {
		return nil, err
	}

	// Loop to check returned messages from KEVM
	// Handle calls for information and return result when done
	for {
		// TODO, need to know how much to read here? Is there a better way?
		_, err = s.server.Read(b)
		if err != nil {
			return nil, err
		}
		vmQuery := &VMQuery{}
		err = proto.Unmarshal(b, vmQuery)
		if err != nil {
			return nil, err
		}
		switch v := vmQuery.Query.(type) {
		case *VMQuery_GetCode:
			addr := common.BytesToAddress(v.GetCode.Address)
			s.stateDB.GetCode(addr)
			// TODO Return code
		case *VMQuery_CallResult:
			return result, err
		case *VMQuery_GetAccount:
		case *VMQuery_GetBlockhash:
		case *VMQuery_GetStorageData:
		}

	}
}

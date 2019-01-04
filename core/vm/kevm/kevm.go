package kevm

import (
	"encoding/binary"
	"errors"
	"log"
	"math/big"
	"net"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
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
	CreateAccount(common.Address)

	SubBalance(common.Address, *big.Int)
	AddBalance(common.Address, *big.Int)
	GetBalance(common.Address) *big.Int

	GetNonce(common.Address) uint64
	SetNonce(common.Address, uint64)

	GetCodeHash(common.Address) common.Hash
	GetCode(common.Address) []byte
	SetCode(common.Address, []byte)
	GetCodeSize(common.Address) int

	GetState(common.Address, common.Hash) common.Hash
	SetState(common.Address, common.Hash, common.Hash)

	AddRefund(uint64)

	Suicide(common.Address) bool

	Exist(common.Address) bool
	Empty(common.Address) bool

	AddLog(*types.Log)
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
	ret, _, leftOverGas, err = s.callCtx(callCtx, false)
	return
}

func (s *Server) Create(caller common.Address, code []byte, gas uint64, value *big.Int) (ret []byte, contractAddr common.Address, leftOverGas uint64, err error) {
	callCtx := &CallContext{}
	callCtx.CallerAddr = caller.Bytes()
	callCtx.InputData = code
	callCtx.CallValue = value.Bytes()
	binary.BigEndian.PutUint64(callCtx.GasProvided, gas)
	ret, contractAddr, leftOverGas, err = s.callCtx(callCtx, true)
	return
}

func (s *Server) callCtx(ctx *CallContext, createContract bool) (ret []byte, contractAddr common.Address, leftOverGas uint64, err error) {
	nilAddr := common.BytesToAddress([]byte("0x0"))
	result, err := s.sendAndReturn(ctx)
	if err != nil {
		return nil, nilAddr, 0, err
	}
	if result.Error {
		// TODO: Should this return a standardized error?
		return nil, nilAddr, 0, errors.New("VM Returned Error")
	}
	// Modify Accounts
	for _, mAccount := range result.ModifiedAccounts {
		mAddr := common.BytesToAddress(mAccount.Address)
		if s.stateDB.Exist(mAddr) {
			bal := new(big.Int)
			bal.SetBytes(mAccount.Balance)
			cur_bal := s.stateDB.GetBalance(mAddr)
			bal.Sub(bal, cur_bal)
			sign := bal.Sign()
			if sign == -1 {
				bal.Neg(bal)
				s.stateDB.SubBalance(mAddr, bal)
			} else if sign == 1 {
				s.stateDB.AddBalance(mAddr, bal)
			}
		} else {
			s.stateDB.CreateAccount(mAddr)
			bal := new(big.Int)
			bal.SetBytes(mAccount.Balance)
			s.stateDB.AddBalance(mAddr, bal)
		}
		s.stateDB.SetNonce(mAddr, binary.BigEndian.Uint64(mAccount.Nonce))
		s.stateDB.SetCode(mAddr, mAccount.Code)
		for _, sUpdate := range mAccount.StorageUpdates {
			key := common.BytesToHash(sUpdate.Offset)
			value := common.BytesToHash(sUpdate.Data)
			s.stateDB.SetState(mAddr, key, value)
		}
	}
	// Delete Accounts
	for _, dAccount := range result.DeletedAccounts {
		dAddr := common.BytesToAddress(dAccount)
		bal := s.stateDB.GetBalance(dAddr)
		s.stateDB.SubBalance(dAddr, bal)
		s.stateDB.Suicide(dAddr)
	}
	// Add Logs
	for _, lg := range result.Logs {
		lgToStore := new(types.Log)
		lgToStore.Address = common.BytesToAddress(lg.Address)
		lgToStore.Data = lg.Data
		for _, tp := range lg.Topics {
			lgToStore.Topics = append(lgToStore.Topics, common.BytesToHash(tp))
		}
		s.stateDB.AddLog(lgToStore)
	}
	s.stateDB.AddRefund(binary.BigEndian.Uint64(result.GasRefund))
	ret = result.ReturnData
	leftOverGas = binary.BigEndian.Uint64(result.GasRemaining)
	// TODO: Determine how to find contract Address
	// Will only run this code when createContract bool is true
	// This is to separate Call from Create
	return ret, nilAddr, leftOverGas, nil
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
			ret := &Code{}
			ret.Code = s.stateDB.GetCode(addr)
			b, err := proto.Marshal(ret)
			_, err = s.server.Write(b)
			if err != nil {
				return nil, err
			}
		case *VMQuery_CallResult:
			return v.CallResult, err
		case *VMQuery_GetAccount:
			addr := common.BytesToAddress(v.GetAccount.Address)
			ret := &Account{}
			if !s.stateDB.Exist(addr) {
				b, err := proto.Marshal(ret)
				_, err = s.server.Write(b)
				if err != nil {
					return nil, err
				}
				break
			}
			ret.Balance = s.stateDB.GetBalance(addr).Bytes()
			binary.BigEndian.PutUint64(ret.Nonce, s.stateDB.GetNonce(addr))
			if s.stateDB.GetCodeSize(addr) == 0 {
				ret.CodeEmpty = true
			} else {
				ret.CodeHash = s.stateDB.GetCodeHash(addr).Bytes()
			}
			b, err := proto.Marshal(ret)
			_, err = s.server.Write(b)
			if err != nil {
				return nil, err
			}
		case *VMQuery_GetBlockhash:
			ret := &Blockhash{}
			ret.Hash = s.getHash(uint64(v.GetBlockhash.Offset)).Bytes()
			b, err := proto.Marshal(ret)
			_, err = s.server.Write(b)
			if err != nil {
				return nil, err
			}
		case *VMQuery_GetStorageData:
			ret := &StorageData{}
			addr := common.BytesToAddress(v.GetStorageData.Address)
			key := common.BytesToHash(v.GetStorageData.Offset)
			ret.Data = s.stateDB.GetState(addr, key).Bytes()
			b, err := proto.Marshal(ret)
			_, err = s.server.Write(b)
			if err != nil {
				return nil, err
			}
		}
	}
}

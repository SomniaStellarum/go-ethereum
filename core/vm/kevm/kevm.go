package kevm

import (
	"log"
	"math/big"
	"net"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/params"
	"github.com/golang/protobuf/proto"
)

type Server struct {
	server net.Conn
}

func NewServer(chainConfig *params.ChainConfig) (s *Server, err error) {
	s.server, err = net.Dial("tcp", "http://localhost:8080")
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
	return ret, leftOverGas, nil
}

func (s *Server) Create(caller common.Address, code []byte, gas uint64, value *big.Int) (ret []byte, contractAddr common.Address, leftOverGas uint64, err error) {
	// TODO: Send CallContext Message
	return ret, contractAddr, leftOverGas, nil
}

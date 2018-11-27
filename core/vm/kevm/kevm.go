package kevm

import "net"

type Server struct {
	server net.Conn
}

func NewServer() (s *Server, err error) {
	s.server, err = net.Dial("tcp", "http://localhost:8080")
	return s, err
}

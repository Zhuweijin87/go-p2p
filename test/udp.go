package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"
)

type node struct {
	IP  string
	UDP int
	TCP int
	ID  string
}
type udp struct {
	conn  *net.UDPConn
	self  node
	close chan struct{}
	tab   *bucket
}

// p2p 中心节点
var Mainnet = []node{
	{"192.168.1.105", 8001, 8001, "111111"},
}

const (
	pingTag = iota + 1
	pongTag
	findTag
	neigbourTag
)

type (
	ping struct {
		ID     string
		Expire int64
	}

	pong struct {
		ID     string
		Expire int64
	}

	find struct {
		ID     string
		Expire int64
	}

	neigbour struct {
		ID     string
		Expire int64
		Nodes  []node
	}
)

type packet interface {
	handle(t *udp, to *net.UDPAddr) error
}

func (p ping) handle(t *udp, to *net.UDPAddr) error {
	log.Println("ping handle")

	reply := &pong{
		ID:     t.self.ID,
		Expire: time.Now().Add(10 * time.Second).Unix(),
	}

	t.send(pongTag, reply, to)

	return nil
}

func (p pong) handle(t *udp, to *net.UDPAddr) error {
	log.Println("pong handle")
	return nil
}

//
func (p find) handle(t *udp, to *net.UDPAddr) error {
	log.Println("find handle:", p.ID, p.Expire)

	var nodes []node
	nodes = append(nodes, t.self)
	reply := &neigbour{
		ID:     "neigbour",
		Expire: time.Now().Add(10 * time.Second).Unix(),
		Nodes:  nodes,
	}
	t.send(neigbourTag, reply, to)
	return nil
}

// 处理返回节点
func (p neigbour) handle(t *udp, to *net.UDPAddr) error {
	log.Println("neogbour handle")

	for _, n := range p.Nodes {
		fmt.Println(":", n.ID, n.IP, n.UDP)
		t.ping(to)
	}
	return nil
}

func ListenUDP(saddr string, id string) *udp {
	laddr, err := net.ResolveUDPAddr("udp", saddr)
	if err != nil {
		log.Println("resolve udp error:", err)
		return nil
	}

	conn, err := net.ListenUDP("udp", laddr)
	if err != nil {
		log.Println("listen udp error:", err)
		return nil
	}

	udp := &udp{
		conn: conn,
		self: node{
			IP:  laddr.IP.String(),
			UDP: laddr.Port,
			TCP: laddr.Port,
			ID:  id,
		},
		close: make(chan struct{}),
		tab:   newBucket(),
	}

	go udp.readLoop()
	go udp.discoverLoop()

	return udp
}

func (t *udp) Wait() {
	<-t.close

	close(t.close)
	t.conn.Close()
}

// 处理数据
func (t *udp) readLoop() {
	buf := make([]byte, 1028)
	for {
		nbytes, from, err := t.conn.ReadFromUDP(buf)
		if err != nil && err != io.EOF {
			log.Println("read error:", err)
			return
		}

		log.Println("from:", from.String())

		fmt.Println("recv:", buf[:nbytes])
		pack := decodePacket(buf[:nbytes])
		if pack == nil {
			log.Println("invalid packet")
			continue
		}
		pack.handle(t, from)
	}
}

// 发现潜在节点
func (t *udp) discoverLoop() {
	for {
		// flash
		select {
		case <-time.After(2 * time.Second):
			if t.self.ID == Mainnet[0].ID {
				//log.Println("self node, continue")
				continue
			}
			toaddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", Mainnet[0].IP, Mainnet[0].UDP))
			if err != nil {
				log.Println("resolve error:", err)
				continue
			}

			t.findnode(toaddr)
		}
	}
}

func decodePacket(data []byte) packet {
	var pack packet
	typ := data[0]
	switch typ {
	case pingTag:
		pack = new(ping)
	case pongTag:
		pack = new(pong)
	case findTag:
		pack = new(find)
	case neigbourTag:
		pack = new(neigbour)
	default:
		log.Println("undefine type")
		return nil
	}

	reader := bytes.NewReader(data[1:])
	decoder := json.NewDecoder(reader)
	err := decoder.Decode(pack)
	if err != nil {
		log.Println("decode error:", err)
		return nil
	}
	return pack
}

func encodePacket(typ byte, pack packet) ([]byte, error) {
	buf := new(bytes.Buffer)
	buf.WriteByte(typ)

	encoder := json.NewEncoder(buf)
	err := encoder.Encode(pack)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (t *udp) send(typ byte, pack packet, toaddr *net.UDPAddr) {
	data, err := encodePacket(typ, pack)
	if err != nil {
		log.Println("encode error:", err)
		return
	}
	t.conn.WriteToUDP(data, toaddr)
}

// 查找节点
func (t *udp) findnode(toaddr *net.UDPAddr) {
	p := &find{
		ID:     "test",
		Expire: time.Now().Add(10 * time.Second).Unix(),
	}

	log.Println("find node")
	t.send(findTag, p, toaddr)
}

func (t *udp) ping(toaddr *net.UDPAddr) {
	p := &ping{
		ID:     t.self.ID,
		Expire: time.Now().Add(10 * time.Second).Unix(),
	}

	t.send(pingTag, p, toaddr)
}

func (t *udp) pending() {

}

// 存储节点的盒子
type bucket struct {
	mux sync.Mutex
	db  map[string]node
}

func newBucket() *bucket {
	return &bucket{
		db: make(map[string]node),
	}
}
func (b *bucket) add(n node) {
	b.mux.Lock()
	defer b.mux.Unlock()

	b.db[n.ID] = n
}

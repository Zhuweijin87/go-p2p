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

// 创建一个node
func newNode(id string, ip *net.UDPAddr) node {
	return node{
		ID:  id,
		IP:  ip.IP.String(),
		UDP: ip.Port,
		TCP: ip.Port,
	}
}

// 获取有效的节点
func (t *udp) getNodesExcept(id string) []node {
	var nodes []node
	if len(t.tab.trustNode) > 0 {
		for k, n := range t.tab.trustNode {
			if k != id {
				nodes = append(nodes, n)
			}
		}
	}

	if len(nodes) < 12 {
		for k, n := range t.tab.potentialNode {
			if k != id {
				nodes = append(nodes, n)
			}
		}
	}
	return nodes
}

func (p ping) handle(t *udp, to *net.UDPAddr) error {
	log.Println("ping handle", "from", p.ID)

	reply := &pong{
		ID:     t.self.ID,
		Expire: time.Now().Add(10 * time.Second).Unix(),
	}

	t.send(pongTag, reply, to)

	return nil
}

func (p pong) handle(t *udp, to *net.UDPAddr) error {
	log.Println("pong handle", "from", p.ID)
	return nil
}

func (p find) handle(t *udp, to *net.UDPAddr) error {
	log.Println("find handle")

	var nodes []node
	nodes = t.getNodesExcept(p.ID) // 返回节点

	reply := &neigbour{
		ID:     t.self.ID,
		Expire: time.Now().Add(10 * time.Second).Unix(),
		Nodes:  nodes,
	}

	t.tab.addPotential(newNode(p.ID, to))
	t.send(neigbourTag, reply, to)
	return nil
}

func (p neigbour) handle(t *udp, to *net.UDPAddr) error {
	log.Println("neogbour handle")

	for _, n := range p.Nodes {
		fmt.Println(">>", n.ID, n.IP, n.UDP)
	}

	go t.bondNode(p.Nodes)

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

func (t *udp) readLoop() {
	buf := make([]byte, 1028)
	for {
		nbytes, from, err := t.conn.ReadFromUDP(buf)
		if err != nil && err != io.EOF {
			log.Println("read error:", err)
			return
		}

		//log.Println("from:", from.String())

		//fmt.Println("recv:", buf[:nbytes])
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

			for _, n := range t.tab.trustNode {
				toaddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", n.IP, n.UDP))
				if err != nil {
					log.Println("resolve error:", err)
					break
				}

				t.findnode(toaddr)
			}
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
		ID:     t.self.ID, // from self
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

// 对潜在节点进行ping/pong
func (t *udp) bondNode(nodes []node) {
	for _, n := range nodes {
		if n.ID != t.self.ID {
			toaddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", n.IP, n.UDP))
			if err != nil {
				log.Println("resolve udp addr error", err)
				continue
			}
			t.ping(toaddr)
		}
	}
}

// 存储节点的盒子
type bucket struct {
	sync.Mutex
	trustNode     map[string]node
	potentialNode map[string]node
}

func newBucket() *bucket {
	b := &bucket{
		trustNode:     make(map[string]node),
		potentialNode: make(map[string]node),
	}

	// 将默认节点添加到信任节点中去
	for _, n := range Mainnet {
		b.trustNode[n.ID] = n
	}

	return b
}

// 添加潜在节点
func (b *bucket) addPotential(n node) {
	b.Lock()
	defer b.Unlock()

	b.potentialNode[n.ID] = n
}

// 添加信任节点
func (b *bucket) addTrustNode(n node) {
	b.Lock()
	defer b.Unlock()

	b.trustNode[n.ID] = n
}

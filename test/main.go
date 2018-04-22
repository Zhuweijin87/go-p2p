package main

import (
	"flag"
	"fmt"
	"log"
)

var (
	id   string
	host string
	port int
	def  bool
)

func init() {
	flag.StringVar(&id, "id", "", "identify of node")
	flag.StringVar(&host, "host", "192.168.1.105", "host")
	flag.IntVar(&port, "port", 0, "net port")
	flag.BoolVar(&def, "d", false, "default node")
}

func main() {
	flag.Parse()

	if def {
		node := Mainnet[0]
		id = node.ID
		host = node.IP
		port = node.UDP
	}

	if id == "" {
		log.Println("error:id not set")
		return
	}

	udp := ListenUDP(fmt.Sprintf("%s:%d", host, port), id)

	log.Println("node start...")
	udp.Wait()
}

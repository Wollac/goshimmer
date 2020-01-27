package main

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	client "github.com/iotaledger/goshimmer/client"
	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/trinary"
	flag "github.com/spf13/pflag"
)

const (
	defaultNode       = "localhost:8080"
	defaultAddress    = "GOSHIMMER9HEARTBEAT"
	addressTrytesSize = consts.HashTrytesSize
)

var (
	node    = flag.String("node", defaultNode, "IP and port of the target node's web API")
	address = flag.String("address", defaultAddress, "address of the heartbeat transaction")
	delay   = flag.Duration("delay", time.Minute, "delay between heartbeats")
)

func main() {
	flag.Parse()

	// validate flags
	tcpAddr, err := net.ResolveTCPAddr("tcp", *node)
	if err != nil {
		log.Fatalf("Invalid node address (%s): %s", *node, err)
	}
	txnAddr, err := trinary.Pad(*address, addressTrytesSize)
	if err != nil {
		log.Fatalf("Invalid transaction address (%s): %s", *address, err)
	}

	// check nodes
	api := client.NewGoShimmerAPI(fmt.Sprintf("http://%s", tcpAddr))
	if err := checkNode(api); err != nil {
		log.Fatalf("Error checking node status: %s", err)
	}

	setupCloseHandler()
	for {
		txnHash, err := heartbeat(api, txnAddr)
		if err != nil {
			log.Panicf("Error issuing heartbeat: %s", err)
		}
		log.Printf("Heartbeat: %s\n", txnHash)

		time.Sleep(*delay)
	}
}

func checkNode(api *client.GoShimmerAPI) error {
	_, err := api.GetNeighbors(false)
	return err
}

func heartbeat(api *client.GoShimmerAPI, txnAddr trinary.Trytes) (trinary.Hash, error) {
	txnHash, err := api.BroadcastData(txnAddr, "")
	if err != nil {
		return "", fmt.Errorf("braodcast data: %w", err)
	}
	resp, err := api.GetTransactionObjectsByHash([]trinary.Hash{txnHash})
	if err != nil {
		return "", fmt.Errorf("get transaction object: %w", err)
	}
	if len(resp) < 0 || txnHash != resp[0].Hash {
		return "", errors.New("broadcast transaction not found on node")
	}
	return txnHash, nil
}

func setupCloseHandler() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		fmt.Println("\rCtrl+C received")
		os.Exit(0)
	}()
}

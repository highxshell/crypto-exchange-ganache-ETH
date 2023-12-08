package main

import (
	"math/rand"
	"time"

	"github.com/highxshell/crypto-exchange/client"
	"github.com/highxshell/crypto-exchange/marketmaker"
	"github.com/highxshell/crypto-exchange/server"
)

func main() {
	go server.StartServer()
	time.Sleep(1 * time.Second)

	c := client.NewClient()

	cfg := marketmaker.Config{
		UserID: 		8888,
		OrderSize: 		10_000_000_000_000_000,
		MinSpread: 		20,
		MakeInterval: 	1 * time.Second,
		SeedOffset: 	40,
		ExchangeClient: c,	
		PriceOffset: 	10,
	}
	maker := marketmaker.NewMarketMaker(cfg)

	maker.Start()

	time.Sleep(2 * time.Second)
	go marketOrderPlacer(c)

	select{}
}

func marketOrderPlacer(c *client.Client) {
	ticker := time.NewTicker(700 * time.Millisecond)
	for {
		randint := rand.Intn(10)
		bid := true
		if randint < 7 {bid = false}

		order := client.PlaceOrderParams{
			UserID: 1,
			Bid: bid,
			Size: 1_000_000_000_000_000,
		}
		_, err := c.PlaceMarketOrder(&order)
		if err != nil {
			panic(err)
		}

		<-ticker.C
	}
}


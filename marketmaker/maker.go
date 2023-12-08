package marketmaker

import (
	"time"

	"github.com/highxshell/crypto-exchange/client"
	"go.uber.org/zap"
)

type Config struct {
	UserID         int64
	OrderSize      float64
	MinSpread      float64
	SeedOffset     float64
	ExchangeClient *client.Client
	MakeInterval	time.Duration
	PriceOffset		float64
}

type MarketMaker struct {
	userID			int64
	orderSize 		float64
	minSpread		float64
	seedOffset 		float64	
	priceOffset		float64
	exchangeClient 	*client.Client
	makeInterval	time.Duration
}

func NewMarketMaker(cfg Config) *MarketMaker {
	return &MarketMaker{
		userID: 		cfg.UserID,
		orderSize: 		cfg.OrderSize,
		minSpread: 		cfg.MinSpread,
		seedOffset: 	cfg.SeedOffset,
		exchangeClient: cfg.ExchangeClient,
		makeInterval: 	cfg.MakeInterval,
		priceOffset: 	cfg.PriceOffset,
	}
}

var(
	logger, _ = zap.NewDevelopment()
	sugar = logger.Sugar()
)

func (mm *MarketMaker) Start() {
	defer logger.Sync()
	sugar.Infow("starting market maker",
		"id", 				mm.userID,
		"orderSize", 		mm.orderSize,
		"makeInterval",		mm.makeInterval,
		"minSpread",		mm.minSpread,
		"priceOffset",		mm.priceOffset,
	)
	go mm.makerLoop()
}

func (mm *MarketMaker) makerLoop() {
	ticker := time.NewTicker(mm.makeInterval)

	for {
		bestBid, err := mm.exchangeClient.GetBestBid()
		if err != nil {
			defer logger.Sync() 
			sugar.Error(err)
			break
		}

		bestAsk, err := mm.exchangeClient.GetBestAsk()
		if err != nil {
			defer logger.Sync() 
			sugar.Error(err)
			break
		}

		if bestAsk.Price == 0 && bestBid.Price == 0 {
			if err := mm.seedMarket(); err != nil {
				defer logger.Sync() 
				sugar.Error(err)
				break
			}
			continue
		}
		if bestBid.Price == 0 {
			bestBid.Price = bestAsk.Price - mm.priceOffset*2
		}

		if bestAsk.Price == 0 {
			bestAsk.Price = bestBid.Price + mm.priceOffset*2
		}

		spread := bestAsk.Price - bestBid.Price

		if spread <= mm.minSpread {
			continue
		}

		if err := mm.placeOrder(true, bestBid.Price+mm.priceOffset); err != nil {
			defer logger.Sync() 
			sugar.Error(err)
			break
		}
		if err := mm.placeOrder(false, bestAsk.Price-mm.priceOffset); err != nil {
			defer logger.Sync() 
			sugar.Error(err)
			break
		}

		<-ticker.C
	}
}

func (mm *MarketMaker) placeOrder(bid bool, price float64) error {
	bidOrder := client.PlaceOrderParams{
		UserID: mm.userID,
		Size: 	mm.orderSize,
		Bid: 	bid,
		Price: 	price,
	}
	_, err := mm.exchangeClient.PlaceLimitOrder(&bidOrder)

	return err
}

func (mm *MarketMaker) seedMarket() error {
	currPrice := simulateFetchCurrentETHPrice()
	defer logger.Sync()
	sugar.Infow("orderbooks empty => seeding market.",
		"currentPrice", 	currPrice,
		"seedOffset", 		mm.seedOffset,
	)

	bidOrder := client.PlaceOrderParams{
		UserID: mm.userID,
		Size: 	mm.orderSize,
		Bid: 	true,
		Price: 	currPrice - mm.seedOffset,
	}
	_, err := mm.exchangeClient.PlaceLimitOrder(&bidOrder)
	if err != nil {
		return err
	}

	askOrder := client.PlaceOrderParams{
		UserID: mm.userID,
		Size: 	mm.orderSize,
		Bid: 	false,
		Price: 	currPrice + mm.seedOffset,
	}
	_, err = mm.exchangeClient.PlaceLimitOrder(&askOrder)
	
	return err
}

// this will simulate a call to an other
// exchange fetching the current ETH
// price so we can offset both for a bid and ask
func simulateFetchCurrentETHPrice() float64 {
	time.Sleep(100 * time.Millisecond)

	return 2231.0
}
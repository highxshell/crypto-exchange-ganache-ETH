package server

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"strconv"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/joho/godotenv"
	"go.uber.org/zap"

	"github.com/highxshell/crypto-exchange/orderbook"
	"github.com/labstack/echo/v4"
)

const(
	MarketOrder OrderType = "MARKET"
	LimitOrder 	OrderType = "LIMIT"
	MarketETH Market = "ETH"
)

type (
	OrderType string
	Market string
	PlaceOrderRequest struct {
		UserID	int64
		Type 	OrderType // limit or market
		Bid 	bool
		Size 	float64
		Price 	float64
		Market 	Market
	}
	Order struct{
		UserID		int64
		ID 			int64
		Price 		float64
		Size 		float64
		Bid 		bool
		Timestamp 	int64
	}
	OrderbookData struct{
		TotalBidVolume 	float64
		TotalAskVolume 	float64
		Asks 			[]*Order
		Bids 			[]*Order
	}
	MatchedOrder struct {
		UserID	int64
		Price 	float64
		Size 	float64
		ID 		int64
	}
	APIError struct {
		Error string
	}
)

var(
	logger, _ = zap.NewDevelopment()
	sugar = logger.Sugar()
)

func StartServer() {
	defer logger.Sync() 
	err := godotenv.Load()
	if err != nil {
		log.Fatal(err)
	}

	s := echo.New()
	s.HTTPErrorHandler = httpErrorHandler

	client, err := ethclient.Dial(os.Getenv("GANACHE_URI"))
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()

	exchangePrivateKey := os.Getenv("EXCHANGE_PK")
	ex, err := NewExchange(exchangePrivateKey, client, ctx)
	if err != nil {
		log.Fatal(err)
	}

	ex.registerUser(os.Getenv("USER_1_PK"), 8888)
	ex.registerUser(os.Getenv("USER_2_PK"), 6667)
	ex.registerUser(os.Getenv("ELON_MUSK_PK"), 1)

	s.POST("/order", ex.handlePlaceOrder)

	s.DELETE("/order/:id", ex.cancelOrder)

	s.GET("/trades/:market", ex.handleGetTrades)
	s.GET("/order/:userID", ex.handleGetOrders)
	s.GET("/book/:market", ex.handleGetBook)
	s.GET("/book/:market/bid", ex.handleGetBestBid)
	s.GET("/book/:market/ask", ex.handleGetBestAsk)


	s.Start(":3000")
}

type User struct {
	ID 			int64
	PrivateKey 	*ecdsa.PrivateKey

}

func NewUser(privateKey string, id int64) *User {
	pk, err := crypto.HexToECDSA(privateKey)
	if err != nil{
		panic(err)
	}

	return &User{
		ID: id,
		PrivateKey: pk,
	}
}

func httpErrorHandler(err error, c echo.Context) {
	fmt.Println(err)
}

type Exchange struct {
	Ctx 		context.Context
	Client 		*ethclient.Client
	mu 			sync.RWMutex
	Users 		map[int64]*User
	// Orders maps a user to his orders
	Orders 		map[int64][]*orderbook.Order
	PrivateKey 	*ecdsa.PrivateKey
	orderbooks 	map[Market]*orderbook.Orderbook
}

func NewExchange(privateKey string, client *ethclient.Client, ctx context.Context) (*Exchange, error) {
	orderbooks := make(map[Market]*orderbook.Orderbook)
	orderbooks[MarketETH] = orderbook.NewOrderBook()

	pk, err := crypto.HexToECDSA(privateKey)
	if err != nil{
		return nil, err
	}

	return &Exchange{
		Ctx: 		ctx,
		Client: 	client,
		Users: 		make(map[int64]*User),
		Orders: 	make(map[int64][]*orderbook.Order),
		PrivateKey: pk,
		orderbooks:	orderbooks,
	}, nil
}

type GetOrdersResponse struct {
	Asks []Order
	Bids []Order
}

func (ex *Exchange) registerUser(pk string, userID int64) {
	user := NewUser(pk, userID)
	ex.Users[userID] = user

	sugar.Infow("new exchange User",
		"id", 		userID,
	)
}

func (ex *Exchange) handleGetTrades(c echo.Context) error {
	market := Market(c.Param("market"))
	ob, ok := ex.orderbooks[market]

	if !ok {
		return c.JSON(http.StatusBadRequest, APIError{"orderbook not found"})
	}

	return c.JSON(http.StatusOK, ob.Trades)
}

func (ex *Exchange) handleGetOrders(c echo.Context) error {
	userIDStr := c.Param("userID")
	userID, err := strconv.Atoi(userIDStr)
	if err != nil{
		return err
	}

	ex.mu.RLock()
	orderbookOrders := ex.Orders[int64(userID)] 
	ordersResp := &GetOrdersResponse{
		Asks: []Order{},
		Bids: []Order{},
	}

	for i := 0; i < len(orderbookOrders); i++ {
		// it could be that the order is getting filled even though itss inclueded in this
		// response. We must double check if the limit is not NIL
		if orderbookOrders[i].Limit == nil {
			fmt.Printf("the limit of the order is NIL %+v\n", orderbookOrders[i])
			continue
		}
		order := Order{
			ID: 		orderbookOrders[i].ID,
			UserID: 	orderbookOrders[i].UserID,
			Price: 		orderbookOrders[i].Limit.Price,
			Size: 		orderbookOrders[i].Size,
			Timestamp: 	orderbookOrders[i].Timestamp,
			Bid: 		orderbookOrders[i].Bid,
		}

		if order.Bid {
			ordersResp.Bids = append(ordersResp.Bids, order)
		} else {
			ordersResp.Asks = append(ordersResp.Asks, order)
		}
	}
	ex.mu.RUnlock()

	return c.JSON(http.StatusOK, ordersResp)
}

func (ex *Exchange) handleGetBook(c echo.Context) error{
	market := Market(c.Param("market"))
	ob, ok := ex.orderbooks[market]

	if !ok {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{"msg":"market not found"})
	}

	orderbookData := OrderbookData{
		TotalBidVolume: ob.BidTotalVolume(),
		TotalAskVolume: ob.AskTotalVolume(),
		Asks: 			[]*Order{},
		Bids: 			[]*Order{},
	}
	for _, limit := range ob.Asks() {
		for _, order := range limit.Orders {
			o := Order{
				UserID: 	order.UserID,
				ID: 		order.ID,
				Price: 		order.Limit.Price,
				Size: 		order.Size,
				Bid: 		order.Bid,
				Timestamp: 	order.Timestamp,
			}
			orderbookData.Asks = append(orderbookData.Asks, &o)
		}	
	}
	for _, limit := range ob.Bids() {
		for _, order := range limit.Orders {
			o := Order{
				UserID: 	order.UserID,
				ID: 		order.ID,
				Price: 		order.Limit.Price,
				Size: 		order.Size,
				Bid: 		order.Bid,
				Timestamp: 	order.Timestamp,
			}
			orderbookData.Bids = append(orderbookData.Bids, &o)
		}	
	}

	return c.JSON(http.StatusOK, orderbookData)
}

type PriceResponse struct {
	Price float64
}

func (ex *Exchange) handleGetBestBid(c echo.Context) error {
	market := Market(c.Param("market"))
	ob := ex.orderbooks[market]
	order := Order{}
	if len(ob.Bids()) == 0 {
		return c.JSON(http.StatusOK, order)
	} 

	bestLimit := ob.Bids()[0]
	bestOrder := bestLimit.Orders[0]

	order.Price = bestLimit.Price
	order.UserID = bestOrder.UserID

	return c.JSON(http.StatusOK, order)
}

func (ex *Exchange) handleGetBestAsk(c echo.Context) error {
	market := Market(c.Param("market"))
	ob := ex.orderbooks[market]
	order := Order{}
	if len(ob.Asks()) == 0 {
		return c.JSON(http.StatusOK, order)
	} 

	bestLimit := ob.Asks()[0]
	bestOrder := bestLimit.Orders[0]

	order.Price = bestLimit.Price
	order.UserID = bestOrder.UserID

	return c.JSON(http.StatusOK, order)
}

func (ex *Exchange) cancelOrder(c echo.Context) error {
	idStr := c.Param("id")
	id, _ := strconv.Atoi(idStr)
	ob := ex.orderbooks[MarketETH]
	order := ob.Orders[int64(id)]
	ob.CancelOrder(order)

	log.Println("order canceled id => ", id)

	return c.JSON(200, map[string]interface{}{"msg":"order deleted"})
}

func (ex *Exchange) handlePlaceMarketOrder(market Market, order *orderbook.Order) ([]orderbook.Match, []*MatchedOrder){
	ob := ex.orderbooks[market]
	matches := ob.PlaceMarketOrder(order)
	matchedOrders := make([]*MatchedOrder, len(matches))
	isBid := false
	if order.Bid {isBid = true}
	totalSizeFilled := 0.0
	sumPrice := 0.0
	for i := 0; i < len(matchedOrders); i++ {
		id := matches[i].Bid.ID
		userID := matches[i].Bid.UserID
		if isBid {
			id 		= matches[i].Ask.ID
			userID 	= matches[i].Ask.UserID
		}
		matchedOrders[i] = &MatchedOrder{
			UserID: userID,
			ID: 	id,
			Size: 	matches[i].SizeFilled,
			Price: 	matches[i].Price,
		}
		totalSizeFilled += matches[i].SizeFilled
		sumPrice += matches[i].Price
	}

	avgPrice := sumPrice / float64(len(matches))

	sugar.Infow("filled market order",
		"avgPrice", 	avgPrice,
		"type", 		order.Type(),
		"size",			totalSizeFilled,
	)

	newOrderMap := make(map[int64][]*orderbook.Order)
	ex.mu.Lock()
	for userID, orderbookOrders := range ex.Orders {
		for i := 0; i < len(orderbookOrders); i++ {
			// if the order is not field we place it in the map copy.
			// this means that the size of the order = 0
			if !orderbookOrders[i].IsFilled() {
				newOrderMap[userID] = append(newOrderMap[userID], orderbookOrders[i])
			}
		}
	}

	ex.Orders = newOrderMap
	ex.mu.Unlock()

	return matches, matchedOrders
}

func (ex *Exchange) handlePlaceLimitOrder(market Market, price float64, order *orderbook.Order) error{
	ob := ex.orderbooks[market]
	ob.PlaceLimitOrder(price, order)

	// keep track of the user orders
	ex.mu.Lock()
	ex.Orders[order.UserID] = append(ex.Orders[order.UserID], order)
	ex.mu.Unlock()
	
	return nil
}

type PlaceOrderResponse struct {
	OrderID int64
}

func (ex *Exchange) handlePlaceOrder(c echo.Context) error {
	var placeOrderData PlaceOrderRequest

	if err := json.NewDecoder(c.Request().Body).Decode(&placeOrderData); err != nil{
		return err
	}

	market := Market(placeOrderData.Market)
	order := orderbook.NewOrder(placeOrderData.Bid, placeOrderData.Size, placeOrderData.UserID)

	// limit orders
	if placeOrderData.Type == LimitOrder {
		if err := ex.handlePlaceLimitOrder(market, placeOrderData.Price, order); err != nil{
			return err
		}
	}

	// market orders
	if placeOrderData.Type == MarketOrder {
		matches, _ := ex.handlePlaceMarketOrder(market, order)

		if err := ex.handleMatches(matches); err != nil{
			return err
		}
	}

	resp := &PlaceOrderResponse{order.ID}

	return c.JSON(200, resp)
}

func (ex *Exchange) handleMatches(matches []orderbook.Match) error {
	for _, match := range matches {
		fromUser, ok := ex.Users[match.Ask.UserID]
		if !ok {
			return fmt.Errorf("user not found: %d", match.Ask.UserID)
		}

		toUser, ok := ex.Users[match.Bid.UserID]
		if !ok {
			return fmt.Errorf("user not found: %d", match.Bid.UserID)
		}

		toAddress := crypto.PubkeyToAddress(toUser.PrivateKey.PublicKey)

		// this is only used for the fees
		// exchangePK := ex.PrivateKey.Public()
		// publicKeyECDSA, ok := exchangePK.(*ecdsa.PublicKey)
		// if !ok {
		// 	return fmt.Errorf("error casting public key to ECDSA")
		// }

		amount := big.NewInt(int64(match.SizeFilled))
		transferETH(ex.Ctx, ex.Client, fromUser.PrivateKey, toAddress, amount)
	}

	return nil
}

func transferETH(ctx context.Context, client *ethclient.Client, fromPK *ecdsa.PrivateKey, to common.Address, amount *big.Int) error {
	publicKey := fromPK.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return fmt.Errorf("error casting public key to ECDSA")
	}

	fromAddress := crypto.PubkeyToAddress(*publicKeyECDSA)
	nonce, err := client.PendingNonceAt(ctx, fromAddress)
	if err != nil {
		return err
	}
	
	gasLimit := uint64(21000)
	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil{
		return err
	}

	tx := types.NewTransaction(nonce, to, amount, gasLimit, gasPrice, nil)
	chainID, err := client.ChainID(ctx)
	if err != nil{
		return err
	}

	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), fromPK)
	if err != nil{
		return err
	}

	return client.SendTransaction(ctx, signedTx)
}
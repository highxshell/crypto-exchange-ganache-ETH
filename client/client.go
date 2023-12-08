package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/highxshell/crypto-exchange/orderbook"
	"github.com/highxshell/crypto-exchange/server"
)

const ENDPOINT="http://localhost:3000"

type Client struct {
	*http.Client
}

func NewClient() *Client {
	return &Client{http.DefaultClient}
}

type PlaceOrderParams struct {
	UserID 		int64
	Bid 		bool
	//price only needed for LIMIT
	Price, Size float64
}

func (c *Client) GetTrades(market string) ([]*orderbook.Trade, error) {
	endpoint := fmt.Sprintf("%s/trades/%s", ENDPOINT, market)
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}

	trades := []*orderbook.Trade{}
	if err := json.NewDecoder(resp.Body).Decode(&trades); err != nil{
		return nil, err
	}

	return trades, nil
}

func (c *Client) GetOrders(userID int64) (*server.GetOrdersResponse, error) {
	endpoint := fmt.Sprintf("%s/order/%d", ENDPOINT, userID)
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil{
		return nil, err
	}

	resp, err := c.Do(req)
	if err != nil{
		return nil, err
	}

	orders := server.GetOrdersResponse{}
	if err := json.NewDecoder(resp.Body).Decode(&orders); err != nil{
		return nil, err
	}

	return &orders, nil
}

func (c *Client) CancelOrder(orderID int64) error {
	endpoint := fmt.Sprintf("%s/order/%d", ENDPOINT, orderID)
	req, err := http.NewRequest(http.MethodDelete, endpoint, nil)
	if err != nil{
		return err
	} 

	_, err = c.Do(req)
	if err != nil{
		return err
	}

	return nil
}

func (c *Client) GetBestBid() (*server.Order, error) {
	endpoint := fmt.Sprintf("%s/book/ETH/bid", ENDPOINT)
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}

	order := &server.Order{}
	if err := json.NewDecoder(resp.Body).Decode(order); err != nil {
		return nil, err
	}

	return order, nil
}

func (c *Client) GetBestAsk() (*server.Order, error) {
	endpoint := fmt.Sprintf("%s/book/ETH/ask", ENDPOINT)
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}

	order := &server.Order{}
	if err := json.NewDecoder(resp.Body).Decode(order); err != nil {
		return nil, err
	}

	return order, nil
}

func (c *Client) PlaceMarketOrder(params *PlaceOrderParams) (*server.PlaceOrderResponse, error) {
	p := &server.PlaceOrderRequest{
		UserID: 	params.UserID,
		Type:		server.MarketOrder,
		Bid: 		params.Bid,
		Size: 		params.Size,
		Market: 	server.MarketETH,
	}
	body, err :=json.Marshal(p)
	if err != nil{
		return nil, err
	} 

	endpoint := ENDPOINT + "/order"
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil{
		return nil, err
	} 

	resp, err := c.Do(req)
	if err != nil{
		return nil, err
	}

	placeOrderResponse := &server.PlaceOrderResponse{}
	if err := json.NewDecoder(resp.Body).Decode(placeOrderResponse); err != nil{
		return nil, err
	}
	defer resp.Body.Close()
	
	return placeOrderResponse, nil
}

func (c *Client) PlaceLimitOrder(params *PlaceOrderParams) (*server.PlaceOrderResponse, error) {
	if params.Size == 0.0 {
		return nil, fmt.Errorf("size cannot be 0 when placing a limit order")
	}

	p := &server.PlaceOrderRequest{
		UserID: 	params.UserID,
		Type:		server.LimitOrder,
		Bid: 		params.Bid,
		Size: 		params.Size,
		Price: 		params.Price,
		Market: 	server.MarketETH,
	}
	body, err :=json.Marshal(p)
	if err != nil{
		return nil, err
	} 

	endpoint := ENDPOINT + "/order"
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil{
		return nil, err
	} 

	resp, err := c.Do(req)
	if err != nil{
		return nil, err
	}

	placeOrderResponse := &server.PlaceOrderResponse{}
	if err := json.NewDecoder(resp.Body).Decode(placeOrderResponse); err != nil{
		return nil, err
	}
	defer resp.Body.Close()
	
	return placeOrderResponse, nil
}
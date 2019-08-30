package bitmart

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"sort"
	"time"

	"github.com/oneiro-ndev/commands/cmd/meic/ots"
	"github.com/oneiro-ndev/ndaumath/pkg/pricecurve"
	ndaumath "github.com/oneiro-ndev/ndaumath/pkg/types"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// An OTS is the bitmart implementation of the OTS interface
type OTS struct {
	Symbol       string
	APIKeyPath   string
	auth         Auth
	statusFilter OrderStatus
}

// compile-time check that we actually do implement that interface
var _ ots.OrderTrackingSystem = (*OTS)(nil)

func (e OTS) UpdateQty(order ots.SellOrder) error {
	fmt.Println("update = ", order)
	err := CancelOrder(&e.auth, order.ID)
	//	err := error(nil)
	if err != nil {
		err = errors.Wrap(err, "cancel order request")
		return err
	}
	fmt.Println("OTS = ", e)
	qty := float64(order.Qty) / 100000000
	fmt.Println("qty = ", qty)
	price := float64(order.Price) / 100000000000
	fmt.Println("price = ", price)
	id, err := PlaceOrder(&e.auth, e.Symbol, "sell", price, qty)
	order.ID = uint64(id)
	//	err = error(nil)
	return err
}

func (e OTS) Delete(order ots.SellOrder) error {
	fmt.Println("delete = ", order)
	return CancelOrder(&e.auth, order.ID)
	// return nil
}

func (e OTS) Submit(order ots.SellOrder) error {
	fmt.Println("submit = ", order)
	fmt.Println("OTS = ", e)
	qty := float64(order.Qty) / 100000000
	fmt.Println("qty = ", qty)
	price := float64(order.Price) / 100000000000
	fmt.Println("price = ", price)
	err := error(nil)
	id, err := PlaceOrder(&e.auth, e.Symbol, "sell", price, qty)
	order.ID = uint64(id)
	return err
}

// Init implements ots.OrderTrackingSystem
func (e OTS) Init(logger logrus.FieldLogger) error {
	fmt.Println("symbol = ", e.Symbol)

	return nil
}

func prettyJSON(bytes []byte) (s string, err error) {
	var obj interface{}
	err = json.Unmarshal(bytes, &obj)
	if err != nil {
		return
	}
	bytes, err = json.MarshalIndent(obj, "", "  ")
	s = string(bytes)
	return
}

// Run implements ots.OrderTrackingSystem
func (e OTS) Run(
	logger logrus.FieldLogger,
	sales chan<- ots.TargetPriceSale,
	updates <-chan ots.UpdateOrders,
	errs chan<- error,
) {
	logger = logger.WithField("ots", "bitmart")

	key, err := LoadAPIKey(e.APIKeyPath)
	if err != nil {
		errs <- errors.Wrap(err, "bitmart ots: loading api key")
	}
	e.auth = NewAuth(key)

	e.statusFilter = OrderStatusFrom("pendingandpartialsuccess")
	logger.WithFields(logrus.Fields{
		"ots":          "bitmart",
		"statusFilter": e.statusFilter,
	}).Debug("setup status filter")

	log.Println("OTS =", e)

	// launch a goroutine to watch the updates channel
	go func() {
		logger = logger.WithField("goroutine", "OTS updates monitor")
		for {
			// notice any update instructions
			upd := <-updates

			logger.WithField("desired stack", upd.Orders).Debug("received update instruction")

			// set exchange appropriate sig digits for Qty and Price for update orders
			for idx := range upd.Orders {
				upd.Orders[idx].Qty = ndaumath.Ndau(math.Round(float64(upd.Orders[idx].Qty/1000000)) * 1000000)
				upd.Orders[idx].Price = pricecurve.Nanocent(math.Round(float64(upd.Orders[idx].Price)/10000000) * 10000000)
			}
			// update the current stack
			log.Println("OTS =", e)
			orders, err := GetOrderHistory(&e.auth, e.Symbol, e.statusFilter)
			if err != nil {
				errs <- errors.Wrap(err, "getting orders")
				return
			}

			curStack := make([]ots.SellOrder, 0, 4)

			// order the current stack from lowest to highest price
			sort.Slice(orders, func(i, j int) bool {
				return orders[i].Price > orders[j].Price
			})

			for i := len(orders) - 1; i >= 0; i-- {
				if orders[i].Side == "sell" {
					fQty := fmt.Sprintf("%f", orders[i].RemainingAmount)
					qty, err := ndaumath.ParseNdau(fQty)
					if err != nil {
						errs <- errors.Wrap(err, "converting remaining amount")
						return
					}

					fPrice := fmt.Sprintf("%f", orders[i].Price)
					price, err := pricecurve.ParseDollars(fPrice)
					if err != nil {
						errs <- errors.Wrap(err, "converting price")
						return
					}

					curStack = append(curStack, ots.SellOrder{
						Qty:   qty,
						Price: price,
						ID:    uint64(orders[i].EntrustID),
					})
				}
			}

			log.Println("curstack =", curStack)
			log.Println("update stack =", upd.Orders)

			err = ots.SynchronizeOrders(curStack, upd.Orders, e.UpdateQty, e.Delete, e.Submit)
			if err != nil {
				errs <- errors.Wrap(err, "synchronizing orders")
			}

		}
	}()

	// make first call to get max trade ID
	var maxTradeID int64
	_, maxTradeID, err = GetTradeHistory(&e.auth, e.Symbol)
	if err != nil {
		errs <- errors.Wrap(err, "get order history")
		return
	}
	log.Println("max trade = ", maxTradeID)
	var trades []Trade
	for {
		trades, maxTradeID, err = GetTradeHistoryAfter(&e.auth, e.Symbol, maxTradeID)
		if err != nil {
			errs <- errors.Wrap(err, "get order history after")
			return
		}
		log.Println("new trades = ", trades)
		var tps = ots.TargetPriceSale{Qty: 0}
		// if there are new trades, loop through them, add them up, and notify IUS of new sales
		if len(trades) > 0 {
			for _, trade := range trades {
				tps.Qty = tps.Qty + trade.Amount
			}
			sales <- tps
		}
		time.Sleep(2 * time.Second)
	}

}

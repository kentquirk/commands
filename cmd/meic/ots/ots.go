package ots

import (
	"sort"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// An OrderTrackingSystem handles all the details associated with an individual
// exchange.
//
// It must make a best-effort attempt to generate a TargetPriceSale
// message as close to real-time as possible after a target price sale, and
// must respond to UpdateOrders messages by adjusting that exchange's open
// sell orders according to the message.
type OrderTrackingSystem interface {
	// Init is used to initialize an OTS instance.
	//
	// It is run synchronously, so can return an error. If an OTS instance
	// fails to initialize, it is excluded from the list of running OTSs.
	Init(logger logrus.FieldLogger) error

	// Run is used to start an OTS instance.
	//
	// The sales channel has a small buffer, but in the event the buffer fills,
	// OTS instances must block until it can add the sale to the channel.
	// Otherwise, a sale could fall through the cracks and never generate
	// an appropriate issuance.
	Run(
		logger logrus.FieldLogger,
		sales chan<- TargetPriceSale,
		updates <-chan UpdateOrders,
		errs chan<- error,
	)

	UpdateQty(order SellOrder) error
	Delete(order SellOrder) error
	Submit(order SellOrder) error
}

// SynchronizeOrders handles the grunt work of diffing out the updates implied
// by a current and desired set of sell orders.
func SynchronizeOrders(
	current, desired []SellOrder,
	updateQty func(SellOrder) error,
	delete func(SellOrder) error,
	submit func(SellOrder) error,
) error {
	// sort the current and desired slices by price
	sort.Slice(current, func(i, j int) bool { return current[i].Price < current[j].Price })
	sort.Slice(desired, func(i, j int) bool { return desired[i].Price < desired[j].Price })

	// in essence, this is a merge sort on current and desired
	ci := 0
	di := 0
	var err error
	for ci < len(current) && di < len(desired) {
		switch {
		case current[ci].Price < desired[di].Price:
			// there is ndau for sale at too low a price
			err = delete(current[ci])
			if err != nil {
				err = errors.Wrap(err, "deleting too low order")
				return err
			}
			ci++
		case current[ci].Price == desired[di].Price:
			// the price is right
			if current[ci].Qty != desired[di].Qty {
				current[ci].Qty = desired[di].Qty
				err = updateQty(current[ci])
				if err != nil {
					err = errors.Wrap(err, "updating Qty")
					return err
				}

			}
			ci++
			di++
		case current[ci].Price > desired[di].Price:
			// we are missing a stack level, place an order at desired level and
			// see if that syncs everything up
			err = submit(desired[di])
			if err != nil {
				err = errors.Wrap(err, "submitting order")
				return err
			}
			di++
		}
	}
	// now remove any extra current orders which haven't been dealt with
	for ; ci < len(current); ci++ {
		err = delete(current[ci])
		if err != nil {
			err = errors.Wrap(err, "deleting extra order")
			return err
		}
	}
	// now add any extra desired orders which haven't been dealt with
	for ; di < len(desired); di++ {
		err = submit(desired[di])
		if err != nil {
			err = errors.Wrap(err, "submitting order")
			return err
		}
	}
	return err
}
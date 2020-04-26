package main

import (
	"fmt"
	"log"
	"math"
	"sync"
	"time"
)

type shelfType int

const (
	hotShelf shelfType = iota
	coldShelf
	frozenShelf
	overflowShelf

	numberInHot     = 10
	numberInCold    = 10
	numberInFrozen  = 10
	numberInOverfow = 15
)

type kitchenInfo struct {
	ordersOnShelves []orderInfo

	hotAvailable      int
	coldAvailable     int
	frozenAvailable   int
	overflowAvailable int

	ordersDelivered            int
	ordersDiscardedAsExpired   int
	ordersDiscardedAsLackPlace int
	ordersTotality             int

	allOrdersPosted       bool
	allOrdersDelivered    bool
	ordersOnShelvesMuxtex sync.Mutex

	orderComing chan orderInfo
}

func (kitchen *kitchenInfo) PostOrder(order orderInfo) {

	if order.Temp != hotTemp && order.Temp != coldTemp && order.Temp != frozenTemp {
		log.Fatalf("Invalid Temp in order:\n%s", order.String())
	}

	fmt.Printf("New arrival order:\n%s", order.String())

	kitchen.ordersTotality++
	kitchen.orderComing <- order
}

func (kitchen *kitchenInfo) AllOrdersArePosted() {
	kitchen.allOrdersPosted = true
}

func (kitchen *kitchenInfo) placeNewOrder(order orderInfo) {

	kitchen.ordersOnShelvesMuxtex.Lock()

	if order.Temp == hotTemp && kitchen.hotAvailable > 0 {
		kitchen.hotAvailable--
		order.OnShelf = hotShelf
		kitchen.ordersOnShelves = append(kitchen.ordersOnShelves, order)
	} else if order.Temp == coldTemp && kitchen.coldAvailable > 0 {
		kitchen.coldAvailable--
		order.OnShelf = coldShelf
		kitchen.ordersOnShelves = append(kitchen.ordersOnShelves, order)
	} else if order.Temp == frozenTemp && kitchen.frozenAvailable > 0 {
		kitchen.frozenAvailable--
		order.OnShelf = frozenShelf
		kitchen.ordersOnShelves = append(kitchen.ordersOnShelves, order)
	} else if kitchen.overflowAvailable > 0 {
		kitchen.overflowAvailable--
		order.OnShelf = overflowShelf
		kitchen.ordersOnShelves = append(kitchen.ordersOnShelves, order)
	} else {
		order.OnShelf = overflowShelf

		// select nearest expired order in overflow shelf can be move to that single temp shelf, if possible
		// or select the nearest expired order whatever temp then to discard it
		indexMoveToSpecialShelf := -1
		indexRemoved := -1
		minShelfLifeToMove := math.MaxFloat64
		minShelfLifeToRemove := math.MaxFloat64
		for i, orderOnShelf := range kitchen.ordersOnShelves {
			if orderOnShelf.OnShelf == overflowShelf {

				if orderOnShelf.RemainShelfLife < minShelfLifeToMove {

					if (orderOnShelf.Temp == hotTemp && kitchen.hotAvailable > 0) ||
						(orderOnShelf.Temp == coldTemp && kitchen.coldAvailable > 0) ||
						(orderOnShelf.Temp == frozenTemp && kitchen.frozenAvailable > 0) {
						indexMoveToSpecialShelf = i
						minShelfLifeToMove = orderOnShelf.RemainShelfLife
					}
				}

				if orderOnShelf.RemainShelfLife < minShelfLifeToRemove {
					indexRemoved = i
					minShelfLifeToRemove = orderOnShelf.RemainShelfLife
				}
			}
		}

		if indexMoveToSpecialShelf == -1 {

			fmt.Printf("Have to discard an order because lack place in shelves:\n%s", kitchen.ordersOnShelves[indexRemoved].String())

			// To avoid remove one order then append another one, just replace the discarded order by new order
			kitchen.ordersOnShelves[indexRemoved] = order

			kitchen.ordersDiscardedAsLackPlace++
		} else {

			fmt.Printf("Move the order from overflow shelf to single temp shelf:\n%s", kitchen.ordersOnShelves[indexMoveToSpecialShelf].String())

			switch kitchen.ordersOnShelves[indexMoveToSpecialShelf].Temp {
			case hotTemp:
				kitchen.ordersOnShelves[indexMoveToSpecialShelf].OnShelf = hotShelf
				kitchen.hotAvailable--
			case coldTemp:
				kitchen.ordersOnShelves[indexMoveToSpecialShelf].OnShelf = coldShelf
				kitchen.coldAvailable--
			case frozenTemp:
				kitchen.ordersOnShelves[indexMoveToSpecialShelf].OnShelf = frozenShelf
				kitchen.frozenAvailable--
			}

			kitchen.ordersOnShelves = append(kitchen.ordersOnShelves, order)
		}

		// Whatever move the nearest expired order to single-temperature from overflow shelf or have to just discard it,
		// overflowAvailable unchanged!!!
	}

	fmt.Printf("Available hot shelf %d,\nAvailable cold shelf %d,\nAvailable frozen shelf %d,\nAvailable overflow shelf %d,\n",
		kitchen.hotAvailable, kitchen.coldAvailable, kitchen.frozenAvailable, kitchen.overflowAvailable)

	kitchen.ordersOnShelvesMuxtex.Unlock()
}

func (kitchen *kitchenInfo) SendCourierPickupOrder() orderInfo {

	kitchen.ordersOnShelvesMuxtex.Lock()
	defer func() {
		kitchen.ordersOnShelvesMuxtex.Unlock()

		fmt.Printf("Available hot shelf %d,\nAvailable cold shelf %d,\nAvailable frozen shelf %d,\nAvailable overflow shelf %d,\n",
			kitchen.hotAvailable, kitchen.coldAvailable, kitchen.frozenAvailable, kitchen.overflowAvailable)
	}()

	// Pick up nearest expired order, avoid to them expired eventually as soon as possible
	minShelfLife := math.MaxFloat64
	minShelfLifeIndex := -1
	for i, order := range kitchen.ordersOnShelves {
		if order.RemainShelfLife < minShelfLife {
			minShelfLife = order.RemainShelfLife
			minShelfLifeIndex = i
		}
	}

	if minShelfLifeIndex >= 0 {

		switch kitchen.ordersOnShelves[minShelfLifeIndex].OnShelf {
		case hotShelf:
			kitchen.hotAvailable++
		case coldShelf:
			kitchen.coldAvailable++
		case frozenShelf:
			kitchen.frozenAvailable++
		case overflowShelf:
			kitchen.overflowAvailable++
		}

		pickedUpOrder := kitchen.ordersOnShelves[minShelfLifeIndex]

		// Remove this order from shelf
		kitchen.ordersOnShelves[minShelfLifeIndex] = kitchen.ordersOnShelves[len(kitchen.ordersOnShelves)-1]
		kitchen.ordersOnShelves = kitchen.ordersOnShelves[:len(kitchen.ordersOnShelves)-1]

		kitchen.ordersDelivered++
		return pickedUpOrder

	} else {
		// return a zero value order, mean that there is no order to fetch.
		return orderInfo{}
	}
}

func (kitchen *kitchenInfo) Run() {

	ticker := time.NewTicker(time.Second)
	for !kitchen.allOrdersPosted || len(kitchen.ordersOnShelves) > 0 {

		select {
		case newOrder := <-kitchen.orderComing:
			kitchen.placeNewOrder(newOrder)
		case <-ticker.C:
			kitchen.checkAndUpdateOrdersStatus()
		}
	}
}

func (kitchen *kitchenInfo) checkAndUpdateOrdersStatus() {
	// Update shelf life of each order in shelves, if there is expired order, discard it.

	kitchen.ordersOnShelvesMuxtex.Lock()
	i := 0
	for _, order := range kitchen.ordersOnShelves {

		var shelfDecayModifier float64
		if order.OnShelf == overflowShelf {
			shelfDecayModifier = 2
		} else {
			shelfDecayModifier = 1
		}

		order.RemainShelfLife = order.RemainShelfLife - order.DecayRate*shelfDecayModifier
		if order.RemainShelfLife > 0 {
			kitchen.ordersOnShelves[i] = order
			i++
		} else {
			kitchen.ordersDiscardedAsExpired++

			switch order.OnShelf {
			case hotShelf:
				kitchen.hotAvailable++
			case coldShelf:
				kitchen.coldAvailable++
			case frozenShelf:
				kitchen.frozenAvailable++
			case overflowShelf:
				kitchen.overflowAvailable++
			}

			fmt.Printf("Discard expired order\n%s", order.String())
		}
	}

	if i < len(kitchen.ordersOnShelves) {
		// Remove orders in slice by truncation
		kitchen.ordersOnShelves = kitchen.ordersOnShelves[:i]
	}

	kitchen.ordersOnShelvesMuxtex.Unlock()
}

func (kitchen *kitchenInfo) Summary() {

	fmt.Printf("Summary:\nTotality orders: %d,\nDelivered orders: %d,\nExpired orders: %d,\nDiscarded orders because lack place in shelves: %d.\n",
		kitchen.ordersTotality, kitchen.ordersDelivered, kitchen.ordersDiscardedAsExpired, kitchen.ordersDiscardedAsLackPlace)
	fmt.Printf("Available hot shelf %d,\nAvailable cold shelf %d,\nAvailable frozen shelf %d,\nAvailable overflow shelf %d,\n",
		kitchen.hotAvailable, kitchen.coldAvailable, kitchen.frozenAvailable, kitchen.overflowAvailable)

	if kitchen.hotAvailable != numberInHot {
		log.Fatalf("hotAvailable(%d) should equal to numberInHot(%d)", kitchen.hotAvailable, numberInHot)
	}
	if kitchen.coldAvailable != numberInCold {
		log.Fatalf("hotAvailable(%d) should equal to numberInHot(%d)", kitchen.coldAvailable, numberInCold)
	}
	if kitchen.frozenAvailable != numberInFrozen {
		log.Fatalf("hotAvailable(%d) should equal to numberInHot(%d)", kitchen.frozenAvailable, numberInFrozen)
	}

	if kitchen.ordersTotality != kitchen.ordersDelivered+kitchen.ordersDiscardedAsExpired+kitchen.ordersDiscardedAsLackPlace {
		log.Fatalln("ordersTotality should equal to ordersDelivered + ordersDiscardedAsExpired + ordersDiscardedAsLackPlace")
	}
}

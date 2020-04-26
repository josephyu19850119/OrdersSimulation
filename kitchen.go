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

	availableOnHotShelf      = 10
	availableOnColdShelf     = 10
	availableOnFrozenShelf   = 10
	availableOnOverflowShelf = 15
)

type kitchenInfo struct {
	ordersOnShelves []orderInfo

	ordersCountOnHotShelf      int
	ordersCountOnColdShelf     int
	ordersCountOnFrozenShelf   int
	ordersCountOnOverflowShelf int

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

	if order.Temp == hotTemp && kitchen.ordersCountOnHotShelf < availableOnHotShelf {
		kitchen.ordersCountOnHotShelf++
		order.OnShelf = hotShelf
		kitchen.ordersOnShelves = append(kitchen.ordersOnShelves, order)
	} else if order.Temp == coldTemp && kitchen.ordersCountOnColdShelf < availableOnColdShelf {
		kitchen.ordersCountOnColdShelf++
		order.OnShelf = coldShelf
		kitchen.ordersOnShelves = append(kitchen.ordersOnShelves, order)
	} else if order.Temp == frozenTemp && kitchen.ordersCountOnFrozenShelf < availableOnFrozenShelf {
		kitchen.ordersCountOnFrozenShelf++
		order.OnShelf = frozenShelf
		kitchen.ordersOnShelves = append(kitchen.ordersOnShelves, order)
	} else if kitchen.ordersCountOnOverflowShelf < availableOnOverflowShelf {
		kitchen.ordersCountOnOverflowShelf++
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

					if (orderOnShelf.Temp == hotTemp && kitchen.ordersCountOnHotShelf < availableOnHotShelf) ||
						(orderOnShelf.Temp == coldTemp && kitchen.ordersCountOnColdShelf < availableOnColdShelf) ||
						(orderOnShelf.Temp == frozenTemp && kitchen.ordersCountOnFrozenShelf < availableOnFrozenShelf) {
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
				kitchen.ordersCountOnHotShelf++
			case coldTemp:
				kitchen.ordersOnShelves[indexMoveToSpecialShelf].OnShelf = coldShelf
				kitchen.ordersCountOnColdShelf++
			case frozenTemp:
				kitchen.ordersOnShelves[indexMoveToSpecialShelf].OnShelf = frozenShelf
				kitchen.ordersCountOnFrozenShelf++
			}

			kitchen.ordersOnShelves = append(kitchen.ordersOnShelves, order)
		}

		// Whatever move the nearest expired order to single-temperature from overflow shelf or have to just discard it,
		// ordersCountOnOverflowShelf unchanged!!!
	}

	fmt.Printf("Count of orders hot shelf %d,\nCount of orders cold shelf %d,\nCount of orders frozen shelf %d,\nCount of orders overflow shelf %d,\n",
		kitchen.ordersCountOnHotShelf, kitchen.ordersCountOnColdShelf, kitchen.ordersCountOnFrozenShelf, kitchen.ordersCountOnOverflowShelf)

	kitchen.ordersOnShelvesMuxtex.Unlock()
}

func (kitchen *kitchenInfo) SendCourierPickupOrder() orderInfo {

	kitchen.ordersOnShelvesMuxtex.Lock()
	defer func() {
		kitchen.ordersOnShelvesMuxtex.Unlock()

		fmt.Printf("Count of orders on hot shelf %d,\nCount of orders on cold shelf %d,\nCount of orders on frozen shelf %d,\nCount of orders on overflow shelf %d,\n",
			kitchen.ordersCountOnHotShelf, kitchen.ordersCountOnColdShelf, kitchen.ordersCountOnFrozenShelf, kitchen.ordersCountOnOverflowShelf)
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
			kitchen.ordersCountOnHotShelf--
		case coldShelf:
			kitchen.ordersCountOnColdShelf--
		case frozenShelf:
			kitchen.ordersCountOnFrozenShelf--
		case overflowShelf:
			kitchen.ordersCountOnOverflowShelf--
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
				kitchen.ordersCountOnHotShelf--
			case coldShelf:
				kitchen.ordersCountOnColdShelf--
			case frozenShelf:
				kitchen.ordersCountOnFrozenShelf--
			case overflowShelf:
				kitchen.ordersCountOnOverflowShelf--
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
	fmt.Printf("Count of orders on hot shelf %d,\nCount of orders on cold shelf %d,\nCount of orders on frozen shelf %d,\nCount of orders on overflow shelf %d,\n",
		kitchen.ordersCountOnHotShelf, kitchen.ordersCountOnColdShelf, kitchen.ordersCountOnFrozenShelf, kitchen.ordersCountOnOverflowShelf)

	if kitchen.ordersCountOnHotShelf != 0 {
		log.Fatalf("Hot shelf should be clear, ordersCountOnHotShelf(%d) should equal to 0", kitchen.ordersCountOnHotShelf)
	}
	if kitchen.ordersCountOnColdShelf != 0 {
		log.Fatalf("Cold shelf should be clear, ordersCountOnColdShelf(%d) should equal to 0", kitchen.ordersCountOnColdShelf)
	}
	if kitchen.ordersCountOnFrozenShelf != 0 {
		log.Fatalf("Frozen shelf should be clear, ordersCountOnFrozenShelf(%d) should equal to 0", kitchen.ordersCountOnFrozenShelf)
	}
	if kitchen.ordersCountOnOverflowShelf != 0 {
		log.Fatalf("Overflow shelf should be clear, ordersCountOnOverflowShelf(%d) should equal to 0", kitchen.ordersCountOnOverflowShelf)
	}

	if kitchen.ordersTotality != kitchen.ordersDelivered+kitchen.ordersDiscardedAsExpired+kitchen.ordersDiscardedAsLackPlace {
		log.Fatalln("ordersTotality should equal to ordersDelivered + ordersDiscardedAsExpired + ordersDiscardedAsLackPlace")
	}
}

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

// Kitchen hold a kitchen's all shelves and posted orders info
// And keep track of numbers of orders for delivered, discarded because of expired or no space
type Kitchen struct {
	ordersOnShelves []Order

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

	orderComing chan Order
}

// PostOrder post a new order to the kitchen by argument
func (kitchen *Kitchen) PostOrder(order Order) {

	if order.InitShelfLife <= 0 {
		log.Fatalf("Invalid Shelf Life in order:\n%s", order.String())
	}

	if order.DecayRate <= 0 {
		log.Fatalf("Invalid Decay Rate in order:\n%s", order.String())
	}

	if order.Temp != hotTemp && order.Temp != coldTemp && order.Temp != frozenTemp {
		log.Fatalf("Invalid Temp in order:\n%s", order.String())
	}

	kitchen.orderComing <- order
}

// AllOrdersArePosted notify kitchen all orders have been posted
func (kitchen *Kitchen) AllOrdersArePosted() {

	close(kitchen.orderComing)
}

// placeNewOrder select a place for new order
func (kitchen *Kitchen) placeNewOrder(order Order) {

	kitchen.ordersOnShelvesMuxtex.Lock()

	fmt.Printf("New arrival order:\n%s", order.String())
	kitchen.ordersTotality++

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
		minRemainShelfLifeTimeToMove := math.MaxFloat64
		minRemainShelfLifeTimeToRemove := math.MaxFloat64
		for i, orderOnShelf := range kitchen.ordersOnShelves {
			if orderOnShelf.OnShelf == overflowShelf {

				remainShelfLifeTime := orderOnShelf.RemainShelfLife / orderOnShelf.DecayRate / 2

				if remainShelfLifeTime < minRemainShelfLifeTimeToMove {

					if (orderOnShelf.Temp == hotTemp && kitchen.ordersCountOnHotShelf < availableOnHotShelf) ||
						(orderOnShelf.Temp == coldTemp && kitchen.ordersCountOnColdShelf < availableOnColdShelf) ||
						(orderOnShelf.Temp == frozenTemp && kitchen.ordersCountOnFrozenShelf < availableOnFrozenShelf) {
						indexMoveToSpecialShelf = i
						minRemainShelfLifeTimeToMove = remainShelfLifeTime
					}
				}

				if remainShelfLifeTime < minRemainShelfLifeTimeToRemove {
					indexRemoved = i
					minRemainShelfLifeTimeToRemove = remainShelfLifeTime
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

	kitchen.ordersOnShelvesMuxtex.Unlock()
}

// SendCourierPickupOrder send a courier to fetch order from kitchen
// Retrun picked up order, if retrun a zero value struct Order, mean that thers is no order wait to pick up in kitchen
func (kitchen *Kitchen) SendCourierPickupOrder() Order {

	kitchen.ordersOnShelvesMuxtex.Lock()
	defer func() {
		kitchen.ordersOnShelvesMuxtex.Unlock()
	}()

	// Pick up nearest expired order, avoid to them expired eventually as soon as possible
	minRemainShelfLifeTime := math.MaxFloat64
	minRemainShelfLifeTimeIndex := -1
	for i, order := range kitchen.ordersOnShelves {
		var remainShelfLifeTime float64
		if order.OnShelf == overflowShelf {
			remainShelfLifeTime = order.RemainShelfLife / order.DecayRate / 2
		} else {
			remainShelfLifeTime = order.RemainShelfLife / order.DecayRate / 1
		}
		if remainShelfLifeTime < minRemainShelfLifeTime {
			minRemainShelfLifeTime = remainShelfLifeTime
			minRemainShelfLifeTimeIndex = i
		}
	}

	if minRemainShelfLifeTimeIndex >= 0 {

		switch kitchen.ordersOnShelves[minRemainShelfLifeTimeIndex].OnShelf {
		case hotShelf:
			kitchen.ordersCountOnHotShelf--
		case coldShelf:
			kitchen.ordersCountOnColdShelf--
		case frozenShelf:
			kitchen.ordersCountOnFrozenShelf--
		case overflowShelf:
			kitchen.ordersCountOnOverflowShelf--
		}

		pickedUpOrder := kitchen.ordersOnShelves[minRemainShelfLifeTimeIndex]

		// Remove this order from shelf
		kitchen.ordersOnShelves[minRemainShelfLifeTimeIndex] = kitchen.ordersOnShelves[len(kitchen.ordersOnShelves)-1]
		kitchen.ordersOnShelves = kitchen.ordersOnShelves[:len(kitchen.ordersOnShelves)-1]

		kitchen.ordersDelivered++
		return pickedUpOrder

	} else {
		// return a zero value order, mean that there is no order to fetch.
		return Order{}
	}
}

// Run let the kitchen start to receive order and courier, and periodic update and check status of orders on shelves
func (kitchen *Kitchen) Run() {

	ticker := time.NewTicker(time.Second)
	for !kitchen.allOrdersPosted || len(kitchen.ordersOnShelves) > 0 {

		select {
		case newOrder := <-kitchen.orderComing:
			if newOrder == (Order{}) {
				kitchen.allOrdersPosted = true
			} else {
				kitchen.placeNewOrder(newOrder)
			}
		case <-ticker.C:
			kitchen.checkAndUpdateOrdersStatus()
		}
	}

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
	if len(kitchen.ordersOnShelves) != 0 {
		log.Fatalf("All orders on shelves should be clear, len(kitchen.ordersOnShelves):%d should equal to 0", kitchen.ordersCountOnOverflowShelf)
	}

	fmt.Printf("Summary:\nTotality orders: %d,\nDelivered orders: %d,\nExpired orders: %d,\nDiscarded orders because lack place in shelves: %d.\n",
		kitchen.ordersTotality, kitchen.ordersDelivered, kitchen.ordersDiscardedAsExpired, kitchen.ordersDiscardedAsLackPlace)

	if kitchen.ordersTotality != kitchen.ordersDelivered+kitchen.ordersDiscardedAsExpired+kitchen.ordersDiscardedAsLackPlace {
		log.Fatalln("ordersTotality should equal to ordersDelivered + ordersDiscardedAsExpired + ordersDiscardedAsLackPlace")
	}
}

// checkAndUpdateOrdersStatus update shelf life of each order in shelves, if there is expired order, discard it.
func (kitchen *Kitchen) checkAndUpdateOrdersStatus() {

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

		if kitchen.ordersCountOnHotShelf+kitchen.ordersCountOnColdShelf+kitchen.ordersCountOnFrozenShelf+kitchen.ordersCountOnOverflowShelf != len(kitchen.ordersOnShelves) {
			log.Fatalf("Sum of ordersCountOnHotShelf(%d), ordersCountOnColdShelf(%d), ordersCountOnFrozenShelf(%d) and ordersCountOnOverflowShelf(%d) should equal to len of ordersOnShelves(%d)",
				kitchen.ordersCountOnHotShelf, kitchen.ordersCountOnColdShelf, kitchen.ordersCountOnFrozenShelf, kitchen.ordersCountOnOverflowShelf, len(kitchen.ordersOnShelves))
		}

		fmt.Printf("Count of orders on hot shelf %d,\nCount of orders on cold shelf %d,\nCount of orders on frozen shelf %d,\nCount of orders on overflow shelf %d,\n",
			kitchen.ordersCountOnHotShelf, kitchen.ordersCountOnColdShelf, kitchen.ordersCountOnFrozenShelf, kitchen.ordersCountOnOverflowShelf)
	}
	kitchen.ordersOnShelvesMuxtex.Unlock()
}

// ShowShelvesStatus print orders count of each shelves and check them are expected, for monitoring status of shelves
func (kitchen *Kitchen) ShowShelvesStatus() {

	kitchen.ordersOnShelvesMuxtex.Lock()

	if kitchen.ordersCountOnHotShelf+kitchen.ordersCountOnColdShelf+kitchen.ordersCountOnFrozenShelf+kitchen.ordersCountOnOverflowShelf != len(kitchen.ordersOnShelves) {
		log.Fatalf("Sum of ordersCountOnHotShelf(%d), ordersCountOnColdShelf(%d), ordersCountOnFrozenShelf(%d) and ordersCountOnOverflowShelf(%d) should equal to len of ordersOnShelves(%d)",
			kitchen.ordersCountOnHotShelf, kitchen.ordersCountOnColdShelf, kitchen.ordersCountOnFrozenShelf, kitchen.ordersCountOnOverflowShelf, len(kitchen.ordersOnShelves))
	}

	fmt.Printf("Count of orders on hot shelf %d,\nCount of orders on cold shelf %d,\nCount of orders on frozen shelf %d,\nCount of orders on overflow shelf %d,\n",
		kitchen.ordersCountOnHotShelf, kitchen.ordersCountOnColdShelf, kitchen.ordersCountOnFrozenShelf, kitchen.ordersCountOnOverflowShelf)

	kitchen.ordersOnShelvesMuxtex.Unlock()
}

package main

import (
	"encoding/json"
	"flag"
	"fmt" // Monitoring info in simulation and final summary output by fmt
	"io/ioutil"
	"log" // Debug and trace info or exception output by log
	"math"
	"math/rand"
	"os"
	"sync"
	"time"
)

type shelfType int

const (
	hotTemp    = "hot"
	coldTemp   = "cold"
	frozenTemp = "frozen"

	hotShelf shelfType = iota
	coldShelf
	frozenShelf
	overflowShelf

	numberInHot     = 10
	numberInCold    = 10
	numberInFrozen  = 10
	numberInOverfow = 15
)

type orderInfo struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Temp      string  `json:"temp"`
	ShelfLife float64 `json:"shelfLife"`
	DecayRate float64 `json:"decayRate"`
	OnShelf   shelfType
}

func (order orderInfo) String() string {

	return fmt.Sprintf("ID:\t\t\t%s\nName:\t\t\t%s\nRemain Shelf Life:\t%g\n", order.ID, order.Name, order.ShelfLife)
}

func loadThenPostOrders(orderComing chan orderInfo, ordersTotality *int) {
	ordersPostedRate := flag.Int("orders-posted-rate", 2, "Orders are posted to kichen rate per second")
	ordersFilePath := flag.String("orders-file-path", "orders.json", "Path of orders file in json")

	flag.Parse()

	log.Printf("Path of orders file: %s\n", *ordersFilePath)

	postInterval := time.Duration(1000 / *ordersPostedRate)
	log.Printf("Order posted interval: %v\n", postInterval)

	ordersFile, err := os.Open(*ordersFilePath)
	defer ordersFile.Close()
	if err != nil {
		log.Fatalln(err)
	}

	ordersInBytes, err := ioutil.ReadAll(ordersFile)
	if err != nil {
		log.Fatalln(err)
	}

	var orders []orderInfo

	err = json.Unmarshal(ordersInBytes, &orders)
	if err != nil {
		log.Fatalln(err)
	}
	for _, order := range orders {
		time.Sleep(time.Millisecond * postInterval)
		orderComing <- order
		(*ordersTotality)++
	}

	// Post an empty ID order indicate all orders are posted
	orderComing <- orderInfo{ID: ""}
}

func sendCourier(courierComing chan bool) {

	rand.Seed(time.Now().UnixNano())

	courierIntervalLower := 2
	courierIntervalUpper := 6
	for {
		// In term of upper and lower bound of courier arriving interval, generate a random interval each time
		interval := time.Duration(courierIntervalLower + rand.Intn(courierIntervalUpper-courierIntervalLower+1))
		time.Sleep(interval * time.Second)

		courierComing <- true
	}
}

func main() {

	orderComing := make(chan orderInfo)
	ordersTotality := 0
	go loadThenPostOrders(orderComing, &ordersTotality)

	courierComing := make(chan bool)
	go sendCourier(courierComing)

	// Periodicity update and check status of orders in shelves per second
	ticker := time.NewTicker(time.Second)

	var ordersOnShelves []orderInfo

	hotAvailable := numberInHot
	coldAvailable := numberInCold
	frozenAvailable := numberInFrozen
	overflowAvailable := numberInOverfow

	allOrdersPosted := false
	var ordersOnShelvesMuxtex sync.Mutex

	ordersDelivered := 0
	ordersDiscardedAsExpired := 0
	ordersDiscardedAsLackPlace := 0

	for {
		select {
		case order := <-orderComing:

			if len(order.ID) == 0 {
				allOrdersPosted = true
				break
			}

			if order.Temp != hotTemp && order.Temp != coldTemp && order.Temp != frozenTemp {
				log.Fatalf("Invalid Temp \"%s\" in order %s", order.Temp, order.ID)
			}

			fmt.Printf("New arrival order:\n%s", order.String())

			ordersOnShelvesMuxtex.Lock()

			if order.Temp == hotTemp && hotAvailable > 0 {
				hotAvailable--
				order.OnShelf = hotShelf
				ordersOnShelves = append(ordersOnShelves, order)
			} else if order.Temp == coldTemp && coldAvailable > 0 {
				coldAvailable--
				order.OnShelf = coldShelf
				ordersOnShelves = append(ordersOnShelves, order)
			} else if order.Temp == frozenTemp && frozenAvailable > 0 {
				frozenAvailable--
				order.OnShelf = frozenShelf
				ordersOnShelves = append(ordersOnShelves, order)
			} else if overflowAvailable > 0 {
				overflowAvailable--
				order.OnShelf = overflowShelf
				ordersOnShelves = append(ordersOnShelves, order)
			} else {
				order.OnShelf = overflowShelf

				// select nearest expired order in overflow shelf can be move to that single temp shelf, if possible
				// or select the nearest expired order whatever temp then to discard it
				indexMoveToSpecialShelf := -1
				indexRemoved := -1
				minShelfLifeToMove := math.MaxFloat64
				minShelfLifeToRemove := math.MaxFloat64
				for i, orderOnShelf := range ordersOnShelves {
					if orderOnShelf.OnShelf == overflowShelf {

						if orderOnShelf.ShelfLife < minShelfLifeToMove {
							if (orderOnShelf.Temp == hotTemp && hotAvailable > 0) ||
								(orderOnShelf.Temp == coldTemp && coldAvailable > 0) ||
								(orderOnShelf.Temp == frozenTemp && frozenAvailable > 0) {
								indexMoveToSpecialShelf = i
								minShelfLifeToMove = orderOnShelf.ShelfLife
							}
						}

						if orderOnShelf.ShelfLife < minShelfLifeToRemove {
							indexRemoved = i
							minShelfLifeToRemove = orderOnShelf.ShelfLife
						}
					}
				}

				if indexMoveToSpecialShelf == -1 {
					fmt.Printf("Have to discard an order because lack place in shelves:\n%s", ordersOnShelves[indexRemoved].String())

					// To avoid remove one order then append another one, just replace the nearest expired by new order
					ordersOnShelves[indexRemoved] = order

					ordersDiscardedAsLackPlace++
				} else {
					switch ordersOnShelves[indexMoveToSpecialShelf].Temp {
					case hotTemp:
						ordersOnShelves[indexMoveToSpecialShelf].OnShelf = hotShelf
						hotAvailable--
					case coldTemp:
						ordersOnShelves[indexMoveToSpecialShelf].OnShelf = coldShelf
						coldAvailable--
					case frozenTemp:
						ordersOnShelves[indexMoveToSpecialShelf].OnShelf = frozenShelf
						frozenAvailable--
					}

					ordersOnShelves = append(ordersOnShelves, order)
				}

				// Whatever move the nearest expired order to single-temperature from overflow shelf or have to just discard it,
				// overflowAvailable unchanged!!!
			}

			fmt.Printf("Available hot shelf %d,\nAvailable cold shelf %d,\nAvailable frozen shelf %d,\nAvailable overflow shelf %d,\n",
				hotAvailable, coldAvailable, frozenAvailable, overflowAvailable)

			ordersOnShelvesMuxtex.Unlock()

		case <-courierComing:

			ordersOnShelvesMuxtex.Lock()

			// Pick up nearest expired order, avoid to them expired eventually as soon as possible
			minShelfLife := math.MaxFloat64
			minShelfLifeIndex := -1
			for i, order := range ordersOnShelves {
				if order.ShelfLife < minShelfLife {
					minShelfLife = order.ShelfLife
					minShelfLifeIndex = i
				}
			}

			if minShelfLifeIndex >= 0 {

				fmt.Printf("Courier take out order:\n%s", ordersOnShelves[minShelfLifeIndex].String())
				switch ordersOnShelves[minShelfLifeIndex].OnShelf {
				case hotShelf:
					hotAvailable++
				case coldShelf:
					coldAvailable++
				case frozenShelf:
					frozenAvailable++
				case overflowShelf:
					overflowAvailable++
				}

				// Check it's not expired indeed
				if ordersOnShelves[minShelfLifeIndex].ShelfLife <= 0 {
					log.Fatalf("Expired order is delivered:\n%s", ordersOnShelves[minShelfLifeIndex].String())
				}

				// Remove this order from shelf
				ordersOnShelves[minShelfLifeIndex] = ordersOnShelves[len(ordersOnShelves)-1]
				ordersOnShelves = ordersOnShelves[:len(ordersOnShelves)-1]

				ordersDelivered++
			} else {
				fmt.Println("There is no order to delivery for current courier")
			}

			fmt.Printf("Available hot shelf %d,\nAvailable cold shelf %d,\nAvailable frozen shelf %d,\nAvailable overflow shelf %d,\n",
				hotAvailable, coldAvailable, frozenAvailable, overflowAvailable)

			ordersOnShelvesMuxtex.Unlock()

		case <-ticker.C:

			ordersOnShelvesMuxtex.Lock()

			if len(ordersOnShelves) > 0 {

				// Update shelf life of each order in shelves, if there is expired order, discard it.
				i := 0
				for _, order := range ordersOnShelves {

					var shelfDecayModifier float64
					if order.OnShelf == overflowShelf {
						shelfDecayModifier = 2
					} else {
						shelfDecayModifier = 1
					}

					order.ShelfLife = order.ShelfLife - order.DecayRate*shelfDecayModifier
					if order.ShelfLife > 0 {
						ordersOnShelves[i] = order
						i++
					} else {
						ordersDiscardedAsExpired++

						switch order.OnShelf {
						case hotShelf:
							hotAvailable++
						case coldShelf:
							coldAvailable++
						case frozenShelf:
							frozenAvailable++
						case overflowShelf:
							overflowAvailable++
						}

						fmt.Printf("Discard expired order\n%s", order.String())
					}
				}

				if i < len(ordersOnShelves) {
					// Remove orders in slice by truncation
					ordersOnShelves = ordersOnShelves[:i]
				}

			} else if allOrdersPosted {
				// If all orders are posted and shelves are clear, complete current simulation
				fmt.Printf("Summary:\nTotality orders: %d,\nDelivered orders: %d,\nExpired orders: %d,\nDiscarded orders because lack place in shelves: %d.\n", ordersTotality, ordersDelivered, ordersDiscardedAsExpired, ordersDiscardedAsLackPlace)
				fmt.Printf("Available hot shelf %d,\nAvailable cold shelf %d,\nAvailable frozen shelf %d,\nAvailable overflow shelf %d,\n",
					hotAvailable, coldAvailable, frozenAvailable, overflowAvailable)

				if hotAvailable != numberInHot {
					log.Fatalf("hotAvailable(%d) should equal to numberInHot(%d)", hotAvailable, numberInHot)
				}
				if coldAvailable != numberInCold {
					log.Fatalf("hotAvailable(%d) should equal to numberInHot(%d)", coldAvailable, numberInCold)
				}
				if frozenAvailable != numberInFrozen {
					log.Fatalf("hotAvailable(%d) should equal to numberInHot(%d)", frozenAvailable, numberInFrozen)
				}

				if ordersTotality != ordersDelivered+ordersDiscardedAsExpired+ordersDiscardedAsLackPlace {
					log.Fatalln("ordersTotality should equal to ordersDelivered + ordersDiscardedAsExpired + ordersDiscardedAsLackPlace")
				}
				return
			}
			ordersOnShelvesMuxtex.Unlock()
		}
	}
}

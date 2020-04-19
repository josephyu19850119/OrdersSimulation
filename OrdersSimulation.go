package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"math/rand"
	"os"
	"sync"
	"time"
)

const (
	Hot = iota
	Cold
	Frozen
	Overflow
)

type Order struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Temp      string  `json:"temp"`
	ShelfLife float64 `json:"shelfLife"`
	DecayRate float64 `json:"decayRate"`
	OnShelf   int
}

func LoadThenPostOrders(orderComing chan Order, ordersTotality *int) {
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

	var orders []Order

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
	orderComing <- Order{ID: ""}
}

func SendCourier(courierComing chan bool) {

	rand.Seed(time.Now().UnixNano())

	courierIntervalLower := 2
	courierIntervalUpper := 6
	for {
		interval := time.Duration(courierIntervalLower + rand.Intn(courierIntervalUpper-courierIntervalLower+1))
		time.Sleep(interval * time.Second)

		courierComing <- true
	}
}
func main() {

	orderComing := make(chan Order)
	ordersTotality := 0
	go LoadThenPostOrders(orderComing, &ordersTotality)

	courierComing := make(chan bool)
	go SendCourier(courierComing)

	ticker := time.NewTicker(time.Second)

	var ordersOnShelves []Order

	hotAvailable := 10
	coldAvailable := 10
	frozenAvailable := 10
	overflowAvailable := 15

	allOrdersPosted := false
	var ordersOnShelvesMuxtex sync.Mutex

	ordersDelivered := 0
	ordersDiscardedAsExpired := 0
	ordersDiscardedAsTooMany := 0

	for {
		select {
		case order := <-orderComing:

			if len(order.ID) == 0 {
				allOrdersPosted = true
				break
			}
			// fmt.Println(order)
			if order.Temp != "hot" && order.Temp != "cold" && order.Temp != "frozen" {
				log.Printf("Invalid Temp in %v\n", order)
				break
			}

			ordersOnShelvesMuxtex.Lock()

			if order.Temp == "hot" && hotAvailable > 0 {
				hotAvailable--
				order.OnShelf = Hot
				ordersOnShelves = append(ordersOnShelves, order)
			} else if order.Temp == "cold" && coldAvailable > 0 {
				coldAvailable--
				order.OnShelf = Cold
				ordersOnShelves = append(ordersOnShelves, order)
			} else if order.Temp == "frozen" && frozenAvailable > 0 {
				frozenAvailable--
				order.OnShelf = Frozen
				ordersOnShelves = append(ordersOnShelves, order)
			} else if overflowAvailable > 0 {
				overflowAvailable--
				order.OnShelf = Overflow
				ordersOnShelves = append(ordersOnShelves, order)
			} else {
				order.OnShelf = Overflow

				indexMoveToSpecialShelf := -1
				indexRemoved := -1
				minShelfLifeToMove := math.MaxFloat64
				minShelfLifeToRemove := math.MaxFloat64
				for i, orderOnShelf := range ordersOnShelves {
					if orderOnShelf.OnShelf == Overflow {

						if orderOnShelf.ShelfLife < minShelfLifeToMove {
							if (orderOnShelf.Temp == "hot" && hotAvailable > 0) ||
								(orderOnShelf.Temp == "cold" && coldAvailable > 0) ||
								(orderOnShelf.Temp == "frozen" && frozenAvailable > 0) {
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
					// To avoid remove one order then append another one, just replace a long-wait by new order
					fmt.Printf("Have to discard an order because too many: %v\n", ordersOnShelves[indexRemoved])
					ordersOnShelves[indexRemoved] = order
					ordersDiscardedAsTooMany++
				} else {
					switch ordersOnShelves[indexMoveToSpecialShelf].Temp {
					case "hot":
						ordersOnShelves[indexMoveToSpecialShelf].OnShelf = Hot
						hotAvailable++
					case "cold":
						ordersOnShelves[indexMoveToSpecialShelf].OnShelf = Cold
						coldAvailable++
					case "frozen":
						ordersOnShelves[indexMoveToSpecialShelf].OnShelf = Frozen
						frozenAvailable++
					}

					ordersOnShelves = append(ordersOnShelves, order)
				}

				// Whatever move an long-wait order to single-temperature from overflow shelf or have to just discard it,
				// overflowAvailable unchanged!!!
			}

			ordersOnShelvesMuxtex.Unlock()

		case <-courierComing:
			// fmt.Println("Courier coming!")
			ordersOnShelvesMuxtex.Lock()

			minShelfLife := math.MaxFloat64
			minShelfLifeIndex := -1
			for i, order := range ordersOnShelves {
				if order.ShelfLife < minShelfLife {
					minShelfLife = order.ShelfLife
					minShelfLifeIndex = i
				}
			}

			if minShelfLifeIndex >= 0 {

				fmt.Printf("Courier take out order: %v\n", ordersOnShelves[minShelfLifeIndex])
				switch ordersOnShelves[minShelfLifeIndex].OnShelf {
				case Hot:
					hotAvailable--
				case Cold:
					coldAvailable--
				case Frozen:
					frozenAvailable--
				case Overflow:
					overflowAvailable--
				}

				if ordersOnShelves[minShelfLifeIndex].ShelfLife <= 0 {
					log.Fatalf("Expired order is delivered: %v\n", ordersOnShelves[minShelfLifeIndex])
				}

				ordersOnShelves[minShelfLifeIndex] = ordersOnShelves[len(ordersOnShelves)-1]
				ordersOnShelves = ordersOnShelves[:len(ordersOnShelves)-1]

				ordersDelivered++
			} else {
				fmt.Println("There is no order to delivery for current courier")
			}

			ordersOnShelvesMuxtex.Unlock()

		case <-ticker.C:
			// fmt.Println("Ticker")
			ordersOnShelvesMuxtex.Lock()

			if len(ordersOnShelves) > 0 {
				i := 0
				for _, order := range ordersOnShelves {

					var shelfDecayModifier float64
					if order.OnShelf == Overflow {
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
						fmt.Printf("[%v] is discarded because of EXPIRE.\n", order)
					}
				}

				if i < len(ordersOnShelves) {

					ordersOnShelves = ordersOnShelves[:i]
				}

				ordersOnShelvesMuxtex.Unlock()
			} else if allOrdersPosted {
				fmt.Printf("Done:\nTotality orders: %d,\nDelivered orders: %d,\nExpired orders: %d,\nDiscarded orders because too many: %d,\n", ordersTotality, ordersDelivered, ordersDiscardedAsExpired, ordersDiscardedAsTooMany)
				return
			}
		}
	}

}

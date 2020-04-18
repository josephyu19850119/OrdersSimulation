package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"math/rand"
	"os"
	"time"
)

const (
	Hot = iota
	Cold
	Frozen
	Overflow
)

func main() {

	type Order struct {
		ID        string  `json:"id"`
		Name      string  `json:"name"`
		Temp      string  `json:"temp"`
		ShelfLife float64 `json:"shelfLife"`
		DecayRate float64 `json:"decayRate"`
		OnShelf   int
	}

	ordersFile, err := os.Open("orders.json")
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

	// fmt.Println(orders)

	rand.Seed(time.Now().UnixNano())

	courierIntervalLower := 2
	courierIntervalUpper := 6

	courierComing := make(chan bool)

	go func() {
		for {
			interval := time.Duration(courierIntervalLower + rand.Intn(courierIntervalUpper-courierIntervalLower+1))
			time.Sleep(interval * time.Second)

			courierComing <- true
		}
	}()

	orderComing := make(chan Order)

	go func() {
		for _, order := range orders {
			time.Sleep(time.Millisecond * 500)
			orderComing <- order
		}
	}()

	ticker := time.NewTicker(time.Second)

	var ordersOnShelves []Order

	hotAvailable := 10
	coldAvailable := 10
	frozenAvailable := 10
	overflowAvailable := 15

	for {
		select {
		case order := <-orderComing:
			// fmt.Println(order)
			if order.Temp != "hot" && order.Temp != "cold" && order.Temp != "frozen" {
				log.Printf("Invalid Temp in %v\n", order)
				break
			}
			// ordersOnShelves = append(ordersOnShelves, order)

			if order.Temp == "hot" && hotAvailable > 0 {
				hotAvailable--
				order.OnShelf = Hot
			} else if order.Temp == "cold" && coldAvailable > 0 {
				coldAvailable--
				order.OnShelf = Cold
			} else if order.Temp == "frozen" && frozenAvailable > 0 {
				frozenAvailable--
				order.OnShelf = Frozen
			} else if overflowAvailable > 0 {
				overflowAvailable--
				order.OnShelf = Overflow
			} else {
				haveToDiscard := hotAvailable == 0 && coldAvailable == 0 && frozenAvailable == 0
				indexToRemoveFromOverflow := -1
				minShelfLife := math.MaxFloat64
				for i, orderOnShelf := range ordersOnShelves {
					if orderOnShelf.OnShelf == Overflow && orderOnShelf.ShelfLife < minShelfLife {

						if haveToDiscard ||
							(orderOnShelf.OnShelf == Hot && hotAvailable > 0) ||
							(orderOnShelf.OnShelf == Cold && coldAvailable > 0) ||
							(orderOnShelf.OnShelf == Frozen && frozenAvailable > 0) {
							indexToRemoveFromOverflow = i
							minShelfLife = orderOnShelf.ShelfLife
						}
					}
				}

				if haveToDiscard {
					// To avoid remove one order then append another one, just replace a long-wait by new order
					ordersOnShelves[indexToRemoveFromOverflow] = order
				} else {
					switch ordersOnShelves[indexToRemoveFromOverflow].Temp {
					case "hot":
						ordersOnShelves[indexToRemoveFromOverflow].OnShelf = Hot
						hotAvailable++
					case "cold":
						ordersOnShelves[indexToRemoveFromOverflow].OnShelf = Cold
						coldAvailable++
					case "frozen":
						ordersOnShelves[indexToRemoveFromOverflow].OnShelf = Frozen
						frozenAvailable++
					}

					// overflowAvailable unchanged
					ordersOnShelves = append(ordersOnShelves, order)
				}

				// Whatever move an long-wait order to single-temperature from overflow shelf or have to just discard it,
				// overflowAvailable unchanged!!!
			}

		case <-courierComing:
			// fmt.Println("Courier coming!")
			minShelfLife := math.MaxFloat64
			minShelfLifeIndex := 0
			for i, order := range ordersOnShelves {
				if order.ShelfLife < minShelfLife {
					minShelfLife = order.ShelfLife
					minShelfLifeIndex = i
				}
			}

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

			ordersOnShelves[minShelfLifeIndex] = ordersOnShelves[len(ordersOnShelves)-1]
			ordersOnShelves = ordersOnShelves[:len(ordersOnShelves)]

		case <-ticker.C:
			// fmt.Println("Ticker")
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
					fmt.Printf("[%v] is discarded because of EXPIRE.\n", order)
				}
			}

			ordersOnShelves = ordersOnShelves[:i]
		}
	}

}

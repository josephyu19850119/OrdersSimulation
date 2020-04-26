package main

import (
	"flag"
	"fmt" // Monitoring info in simulation and final summary output by fmt
	"log" // Debug and trace info or exception output by log
	"math/rand"
	"time"
)

const (
	courierIntervalLower = 2
	courierIntervalUpper = 6
)

func main() {

	ordersPostedRate := flag.Int("orders-posted-rate", 2, "Orders are posted to kichen rate per second")
	ordersFilePath := flag.String("orders-file-path", "orders.json", "Path of orders file in json")

	flag.Parse()

	log.Printf("Path of orders file: %s\n", *ordersFilePath)

	postInterval, err := time.ParseDuration(fmt.Sprintf("%dms", 1000 / *ordersPostedRate))
	if err != nil {
		log.Fatalln(err)
	}

	log.Printf("Order posted interval: %v\n", postInterval)

	allOrders := loadOrders(*ordersFilePath)

	kitchen := kitchenInfo{
		hotAvailable:               numberInHot,
		coldAvailable:              numberInCold,
		frozenAvailable:            numberInFrozen,
		overflowAvailable:          numberInOverfow,
		ordersDelivered:            0,
		ordersDiscardedAsLackPlace: 0,
		ordersDiscardedAsExpired:   0,
		ordersTotality:             0,
		allOrdersPosted:            false,
		orderComing:                make(chan orderInfo)}

	go func() {

		for _, order := range allOrders {
			time.Sleep(postInterval)
			order.RemainShelfLife = order.InitShelfLife
			kitchen.PostOrder(order)
		}

		kitchen.AllOrdersArePosted()
	}()

	go func() {

		rand.Seed(time.Now().UnixNano())
		for {
			// In term of upper and lower bound of courier arriving interval, generate a random interval each time
			interval := time.Duration(courierIntervalLower + rand.Intn(courierIntervalUpper-courierIntervalLower+1))
			time.Sleep(interval * time.Second)

			pickedUpOrder := kitchen.SendCourierPickupOrder()
			if pickedUpOrder != (orderInfo{}) {

				// Check it's not expired indeed
				if pickedUpOrder.RemainShelfLife <= 0 {
					log.Fatalf("Expired order picked!\n%s", pickedUpOrder.String())
				}

				fmt.Printf("Take out order:\n%s", pickedUpOrder.String())
			} else {
				fmt.Println("There is no order to delivery for current courier")
			}
		}
	}()

	kitchen.Run()
	kitchen.Summary()
}

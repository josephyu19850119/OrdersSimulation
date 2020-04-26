package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
)

const (
	hotTemp    = "hot"
	coldTemp   = "cold"
	frozenTemp = "frozen"
)

// Order hold an order all info, include const data member ID, Name, Temp, InitShelfLife, DecayRate
// and running time variable data member RemainShelfLife, On which Shelf
type Order struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	Temp            string  `json:"temp"`
	InitShelfLife   float64 `json:"shelfLife"`
	RemainShelfLife float64
	DecayRate       float64 `json:"decayRate"`
	OnShelf         shelfType
}

// Orders.String print Order's info as string
func (order *Order) String() string {

	return fmt.Sprintf("ID:\t%s\nName:\t%s\nTemp:\t%s\nValue:\t%g\n", order.ID, order.Name, order.Temp, order.RemainShelfLife/order.InitShelfLife)
}

// LoadOrders load orders from file path ordersFilePath
func LoadOrders(ordersFilePath string) []Order {

	ordersFile, err := os.Open(ordersFilePath)
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

	return orders
}

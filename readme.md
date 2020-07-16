Product requirements document in Orders_Simulation.pdf

orders_simulation is built by Go.

In environment with Go, you can run it by "go run orders_simulation.go order.go kitchen.go", along with test data "orders.json" in same directory, or build it at first by run "go build" in the directory then run built bin program.

You can config more arguments to it. Detail is shown by cmd like ".\orders_simulation.exe -h"

You can monitoring process of simulation and final summary info by different arguments and test data.

In orders_simulation, the policy of pick up and move or remove order from overflow shelf, is select nearest expired one always, to as far as possible avoid order is expired eventually.

orders_simulation is self-contained testing, if run into unexpected case, log.Fatalf func will print detail and terminated simulation.

Requirements claim that show "a full listing of shelves'contents" in simulation trace, but it's too tediously to read. So this program just output number of orders in each shelf.
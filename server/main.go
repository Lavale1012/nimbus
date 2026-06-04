// main.go is the entry point for the Nimbus API server.
// It simply calls InitServer(), which wires everything up and starts listening.
package main

import (
	"github.com/nimbus/api/server-init"
)

func main() {
	err := server.InitServer()
	if err != nil {
		panic(err)
	}
}

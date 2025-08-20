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

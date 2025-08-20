package main

import (
	"github.com/nimbus/api/server-init"
)

func main() {
	_, err := server.InitServer()
	if err != nil {
		panic(err)
	}
}

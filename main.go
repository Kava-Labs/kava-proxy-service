// TODO:
package main

import (
	"log"
	"time"
)

func main() {
	for {
		log.Println("There and back again")

		time.Sleep(2 * time.Second)
	}
}

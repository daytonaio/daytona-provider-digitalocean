// main.go
package main

import (
	"fmt"
	"log"

	"github.com/daytonaio/daytona-provider-digitalocean/main_test/util"
)

func main() {
	optionsJson := `{"name":"my-droplet","region":"nyc3","size":"s-1vcpu-1gb","image":"ubuntu-22-04-x64","userData":"#cloud-config\npackages:\n - nginx"}`

	// Create the droplet with the target options
	droplet, err := util.CreateDroplet(optionsJson)
	if err != nil {
		log.Fatalf("Error creating droplet: %v", err)
	}

	fmt.Println(droplet)

}

package main

import (
	"fmt"
	"log"

	"github.com/msn60/isotcpdump/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config load error: %v", err)
	}

	app := config.NewApp(cfg)
	config.Print(cfg)
	fmt.Println(app)

}

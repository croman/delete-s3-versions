package main

import (
	"log"
	"os"

	"github.com/croman/delete-s3-versions/config"
	"github.com/croman/delete-s3-versions/versions"
)

func main() {
	c, err := config.GetConfig()
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	if c == nil {
		os.Exit(0)
	}

	s3Versions := versions.New(c)
	err = s3Versions.Delete()
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
}

package config

import (
	"os"

	flags "github.com/jessevdk/go-flags"
)

// Config command line flags
type Config struct {
	S3Region     string `short:"r" long:"s3-region" default:"eu-west-1" description:"The S3 region"`
	S3DisableSSL string `short:"s" long:"s3-disable-ssl" default:"false" description:"Disable SSL with S3"`
	S3Endpoint   string `short:"e" long:"s3-endpoint" description:"S3 endpoint"`

	BucketName    string `short:"b" long:"bucket" required:"true" description:"The bucket name to check. Use '*' to check all buckets"`
	VersionsCount int    `short:"n" long:"count" required:"true" description:"How many versions to keep"`
	Confirm       bool   `long:"confirm" description:"By default it prints details for the files to be deleted, enabling this flag leads to real S3 file changes"`
}

// GetConfig get application config
func GetConfig() (*Config, error) {
	var config Config
	var parser = flags.NewParser(&config, flags.HelpFlag)

	if _, err := parser.Parse(); err != nil {
		parser.WriteHelp(os.Stdout)
		if flagsErr, ok := err.(*flags.Error); ok && flagsErr.Type == flags.ErrHelp {
			return nil, nil
		}
		return nil, err
	}

	return &config, nil
}

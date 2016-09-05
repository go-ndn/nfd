package main

import (
	"encoding/json"
	"flag"
	"os"

	"github.com/go-ndn/mux"
)

type context struct {
	Listen []struct {
		Network, Address string
	}
	NDNCertPath []string
	Rule        []*mux.VerifyRule

	Debug      bool   `json:"-"`
	ConfigPath string `json:"-"`
}

func background() (*context, error) {
	var ctx context
	flag.StringVar(&ctx.ConfigPath, "config", "nfd.json", "config path")
	flag.BoolVar(&ctx.Debug, "debug", false, "enable logging")

	flag.Parse()

	// config
	configFile, err := os.Open(ctx.ConfigPath)
	if err != nil {
		return nil, err
	}
	defer configFile.Close()

	err = json.NewDecoder(configFile).Decode(&ctx)
	if err != nil {
		return nil, err
	}

	return &ctx, nil
}

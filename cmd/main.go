package main

import (
	"context"
	"io/ioutil"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/mapleFU/fetcher"

	"gopkg.in/yaml.v2"
)

type Config struct {
	Bounds  []fetcher.Bound
	Address []fetcher.DBAddress
}

func ParseConfig(cfg string) *Config {
	f, err := os.Open(cfg)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	b, err := ioutil.ReadAll(f)
	if err != nil {
		panic(err)
	}

	dataMap := struct {
		Bounds  []map[string]string
		Address []fetcher.DBAddress
	}{}
	err = yaml.Unmarshal(b, dataMap)
	if err != nil {
		panic(err)
	}

	c := Config{}
	c.Address = dataMap.Address

	for _, v := range dataMap.Bounds {
		tp, e := v["type"]
		if !e {
			log.Warn("`Bounds` should have a type")
			continue
		}
		switch tp {
		case "speed":
			deltaSecs, err := strconv.Atoi(v["DeltaSecs"])
			if err != nil {
				log.Warn("DeltaSecs should be an integer")
				continue
			}
			deltaMB, err := strconv.Atoi(v["DeltaMB"])
			if err != nil {
				log.Warn("DeltaMB should be an integer")
				continue
			}
			c.Bounds = append(c.Bounds, fetcher.NewSpeedBound(uint64(deltaSecs), uint64(deltaMB)))
		case "quantity":
			prop, err := strconv.ParseFloat(v["proportion"], 64)
			if err != nil {
				log.Warn("proportion should be an float")
				continue
			}
			c.Bounds = append(c.Bounds, fetcher.NewQuantityBound(prop))
		}
	}
	return &c
}

func main() {
	cfg := ParseConfig("config.yaml")

	ctx, cancel := context.WithCancel(context.TODO())

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)

	go func() {
		_ = <-sigc
		cancel()
	}()

	Run(ctx, cfg.Bounds, cfg.Address)
}

func Run(ctx context.Context, bounds []fetcher.Bound, address []fetcher.DBAddress) {
	for _, bound := range bounds {
		currentBound := bound
		go func() {
			for true {
				select {
				case <-time.After(currentBound.CheckDuration()):
					currentBound.Record(address)
				case <-ctx.Done():
					break
				}
			}
		}()
	}
	select {
	case <-ctx.Done():
	}
}

/*
Copyright 2020 DigitalOcean

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/digitalocean/csi-digitalocean/driver"
)

func main() {
	var (
		endpoint     = flag.String("endpoint", "unix:///var/lib/kubelet/plugins/"+driver.DefaultDriverName+"/csi.sock", "CSI endpoint.")
		token        = flag.String("token", "", "DigitalOcean access token.")
		url          = flag.String("url", "https://api.digitalocean.com/", "DigitalOcean API URL.")
		isController = flag.Bool("controller-mode", true, "Run driver with controller mode.")
		isNode       = flag.Bool("node-mode", true, "Run driver with node mode.")
		region       = flag.String("region", "", "DO region slug. Required when running in controller only mode. Don't use if running in Node Mode.")
		doTag        = flag.String("do-tag", "", "Tag DigitalOcean volumes on Create/Attach.")
		driverName   = flag.String("driver-name", driver.DefaultDriverName, "Name for the driver.")
		debugAddr    = flag.String("debug-addr", "", "Address to serve the HTTP debug server on.")
		version      = flag.Bool("version", false, "Print the version and exit.")
	)
	flag.Parse()

	if *version {
		fmt.Printf("%s - %s (%s)\n", driver.GetVersion(), driver.GetCommit(), driver.GetTreeState())
		os.Exit(0)
	}

	if *isController && *token == "" {
		log.Fatalln("token required when running with controller-mode")
	}

	drv, err := driver.NewDriver(*endpoint, *token, *url, *isController, *isNode, *region, *doTag, *driverName, *debugAddr)
	if err != nil {
		log.Fatalln(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-c
		cancel()
	}()

	if err := drv.Run(ctx); err != nil {
		log.Fatalln(err)
	}
}

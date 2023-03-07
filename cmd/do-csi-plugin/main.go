/*
Copyright 2022 DigitalOcean

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
		endpoint               = flag.String("endpoint", "unix:///var/lib/kubelet/plugins/"+driver.DefaultDriverName+"/csi.sock", "CSI endpoint.")
		token                  = flag.String("token", "", "DigitalOcean access token.")
		url                    = flag.String("url", "https://api.digitalocean.com/", "DigitalOcean API URL.")
		region                 = flag.String("region", "", "DigitalOcean region slug. Specify only when running in controller mode outside of a DigitalOcean droplet.")
		doTag                  = flag.String("do-tag", "", "Tag DigitalOcean volumes on Create/Attach.")
		driverName             = flag.String("driver-name", driver.DefaultDriverName, "Name for the driver.")
		debugAddr              = flag.String("debug-addr", "", "Address to serve the HTTP debug server on.")
		defaultVolumesPageSize = flag.Uint("default-volumes-page-size", 0, "The default page size used when paging through volumes results (default: do not specify and let the DO API choose)")
		doAPIRateLimitQPS      = flag.Float64("do-api-rate-limit", 0, "Impose QPS rate limit on DigitalOcean API usage (default: do not rate limit)")
		validateAttachment     = flag.Bool("validate-attachment", false, "Validate if the attachment has fully completed before formatting/mounting the device")
		version                = flag.Bool("version", false, "Print the version and exit.")
	)
	flag.Parse()

	if *version {
		fmt.Printf("%s - %s (%s)\n", driver.GetVersion(), driver.GetCommit(), driver.GetTreeState())
		os.Exit(0)
	}

	if *token == "" && *region != "" {
		log.Fatalln("region flag must not be set when driver is running in node mode (i.e., token flag is unset)")
	}

	drv, err := driver.NewDriver(driver.NewDriverParams{
		Endpoint:               *endpoint,
		Token:                  *token,
		URL:                    *url,
		Region:                 *region,
		DOTag:                  *doTag,
		DriverName:             *driverName,
		DebugAddr:              *debugAddr,
		DefaultVolumesPageSize: *defaultVolumesPageSize,
		DOAPIRateLimitQPS:      *doAPIRateLimitQPS,
		ValidateAttachment:     *validateAttachment,
	})
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

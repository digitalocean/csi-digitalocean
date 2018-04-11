package main

import (
	"flag"
	"log"

	"github.com/digitalocean/csi-digitalocean/driver"
)

func main() {
	var (
		endpoint = flag.String("endpoint", "unix:///var/lib/kubelet/plugins/com.digitalocean.csi.dobs/csi.sock", "CSI endpoint")
		token    = flag.String("token", "", "DigitalOcean access token")
	)

	flag.Parse()

	drv, err := driver.NewDriver(*endpoint, *token)
	if err != nil {
		log.Fatalln(err)
	}

	if err := drv.Run(); err != nil {
		log.Fatalln(err)
	}
}

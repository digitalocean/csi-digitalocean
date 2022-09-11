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

package driver

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/digitalocean/go-metadata"
	"github.com/digitalocean/godo"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
)

const (
	// DefaultDriverName defines the name that is used in Kubernetes and the CSI
	// system for the canonical, official name of this plugin
	DefaultDriverName = "dobs.csi.digitalocean.com"
)

var (
	gitTreeState = "not a git tree"
	commit       string
	version      string
)

// Driver implements the following CSI interfaces:
//
//   csi.IdentityServer
//   csi.ControllerServer
//   csi.NodeServer
//
type Driver struct {
	name         string
	endpoint     string
	debugAddr    string
	isController bool

	srv     *grpc.Server
	httpSrv *http.Server
	log     *logrus.Entry

	// ready defines whether the driver is ready to function. This value will
	// be used by the `Identity` service via the `Probe()` method.
	readyMu sync.Mutex // protects ready
	ready   bool

	csi.NodeServer
	csi.ControllerServer
}

// NewDriverParams defines the parameters that can be passed to NewDriver.
type NewDriverParams struct {
	Endpoint               string
	Token                  string
	URL                    string
	Region                 string
	DOTag                  string
	DriverName             string
	DebugAddr              string
	DefaultVolumesPageSize uint
}

// NewDriver returns a CSI plugin that contains the necessary gRPC
// interfaces to interact with Kubernetes over unix domain sockets for
// managing DigitalOcean Block Storage
func NewDriver(p NewDriverParams) (*Driver, error) {
	driverName := p.DriverName
	if driverName == "" {
		driverName = DefaultDriverName
	}

	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: p.Token,
	})
	oauthClient := oauth2.NewClient(context.Background(), tokenSource)

	mdClient := metadata.NewClient()
	var region string
	if p.Region == "" {
		var err error
		region, err = mdClient.Region()
		if err != nil {
			return nil, fmt.Errorf("couldn't get region from metadata: %s (are you running outside of a DigitalOcean droplet and possibly forgot to specify the 'region' flag?)", err)
		}
	}
	hostIDInt, err := mdClient.DropletID()
	if err != nil {
		return nil, fmt.Errorf("couldn't get droplet ID from metadata: %s (are you running outside of a DigitalOcean droplet?)", err)
	}
	hostID := strconv.Itoa(hostIDInt)

	log := logrus.New().WithFields(logrus.Fields{
		"region":  region,
		"host_id": hostID,
		"version": version,
	})

	var driver *Driver
	// we're assuming only the controller has a non-empty token.
	if p.Token != "" {
		var opts []godo.ClientOpt
		opts = append(opts, godo.SetBaseURL(p.URL))

		if version == "" {
			version = "dev"
		}
		opts = append(opts, godo.SetUserAgent("csi-digitalocean/"+version))

		doClient, err := godo.New(oauthClient, opts...)
		if err != nil {
			return nil, fmt.Errorf("couldn't initialize DigitalOcean client: %s", err)
		}

		healthChecker := NewHealthChecker(&doHealthChecker{account: doClient.Account})

		controller := &Controller{
			publishInfoVolumeName:  driverName + "/volume-name",
			region:                 region,
			doTag:                  p.DOTag,
			defaultVolumesPageSize: p.DefaultVolumesPageSize,

			storage:        doClient.Storage,
			storageActions: doClient.StorageActions,
			droplets:       doClient.Droplets,
			snapshots:      doClient.Snapshots,
			account:        doClient.Account,
			tags:           doClient.Tags,

			healthChecker: healthChecker,
			log:           log,
		}

		driver = &Driver{
			name:         driverName,
			endpoint:     p.Endpoint,
			debugAddr:    p.DebugAddr,
			isController: p.Token != "",

			ControllerServer: controller,
		}
	} else {
		node := &Node{
			publishInfoVolumeName: driverName + "/volume-name",
			region:                region,
			hostID:                func() string { return hostID },
			log:                   log,
			mounter:               newMounter(log),
		}

		driver = &Driver{
			name:         driverName,
			endpoint:     p.Endpoint,
			debugAddr:    p.DebugAddr,
			isController: p.Token != "",

			NodeServer: node,
		}
	}

	return driver, nil
}

// Run starts the CSI plugin by communication over the given endpoint
func (d *Driver) Run(ctx context.Context) error {
	u, err := url.Parse(d.endpoint)
	if err != nil {
		return fmt.Errorf("unable to parse address: %q", err)
	}

	grpcAddr := path.Join(u.Host, filepath.FromSlash(u.Path))
	if u.Host == "" {
		grpcAddr = filepath.FromSlash(u.Path)
	}

	// CSI plugins talk only over UNIX sockets currently
	if u.Scheme != "unix" {
		return fmt.Errorf("currently only unix domain sockets are supported, have: %s", u.Scheme)
	}
	// remove the socket if it's already there. This can happen if we
	// deploy a new version and the socket was created from the old running
	// plugin.
	d.log.WithField("socket", grpcAddr).Info("removing socket")
	if err := os.Remove(grpcAddr); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove unix domain socket file %s, error: %s", grpcAddr, err)
	}

	grpcListener, err := net.Listen(u.Scheme, grpcAddr)
	if err != nil {
		return fmt.Errorf("failed to listen: %v", err)
	}

	// log response errors for better observability
	errHandler := func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		resp, err := handler(ctx, req)
		if err != nil {
			d.log.WithError(err).WithField("method", info.FullMethod).Error("method failed")
		}
		return resp, err
	}

	d.srv = grpc.NewServer(grpc.UnaryInterceptor(errHandler))
	csi.RegisterIdentityServer(d.srv, d)

	// warn the user, it'll not propagate to the user but at least we see if
	// something is wrong in the logs. Only check if the driver is running with
	// a token (i.e: controller)
	if d.isController {
		controller := d.ControllerServer.(*Controller)
		details, err := controller.checkLimit(context.Background())
		if err != nil {
			return fmt.Errorf("failed to check volumes limits on startup: %s", err)
		}
		if details != nil {
			d.log.WithFields(logrus.Fields{
				"limit":       details.limit,
				"num_volumes": details.numVolumes,
			}).Warn("CSI plugin will not function correctly, please resolve volume limit")
		}

		if d.debugAddr != "" {
			mux := http.NewServeMux()
			mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
				err := controller.healthChecker.Check(r.Context())
				if err != nil {
					d.log.WithError(err).Error("executing health check")
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				w.WriteHeader(http.StatusOK)
			})
			d.httpSrv = &http.Server{
				Addr:    d.debugAddr,
				Handler: mux,
			}
		}
		csi.RegisterControllerServer(d.srv, controller)
	} else {
		csi.RegisterNodeServer(d.srv, d.NodeServer.(*Node))
	}

	d.ready = true // we're now ready to go!
	d.log.WithFields(logrus.Fields{
		"grpc_addr": grpcAddr,
		"http_addr": d.debugAddr,
	}).Info("starting server")

	var eg errgroup.Group
	if d.httpSrv != nil {
		eg.Go(func() error {
			<-ctx.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			return d.httpSrv.Shutdown(ctx)
		})
		eg.Go(func() error {
			err := d.httpSrv.ListenAndServe()
			if err == http.ErrServerClosed {
				return nil
			}
			return err
		})
	}
	eg.Go(func() error {
		go func() {
			<-ctx.Done()
			d.log.Info("server stopped")
			d.readyMu.Lock()
			d.ready = false
			d.readyMu.Unlock()
			d.srv.GracefulStop()
		}()
		return d.srv.Serve(grpcListener)
	})

	return eg.Wait()
}

// When building any packages that import version, pass the build/install cmd
// ldflags like so:
//   go build -ldflags "-X github.com/digitalocean/csi-digitalocean/driver.version=0.0.1"

// GetVersion returns the current release version, as inserted at build time.
func GetVersion() string {
	return version
}

// GetCommit returns the current commit hash value, as inserted at build time.
func GetCommit() string {
	return commit
}

// GetTreeState returns the current state of git tree, either "clean" or
// "dirty".
func GetTreeState() string {
	return gitTreeState
}

/*
Copyright 2018 DigitalOcean

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
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/container-storage-interface/spec/lib/go/csi"
	metadata "github.com/digitalocean/go-metadata"
	"github.com/digitalocean/godo"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	"google.golang.org/grpc"
)

const (
	// DriverName defines the name that is used in Kubernetes and the CSI
	// system for the canonical, official name of this plugin
	DriverName = "dobs.csi.digitalocean.com"
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
	endpoint string
	nodeId   string
	region   string

	srv     *grpc.Server
	log     *logrus.Entry
	mounter Mounter

	storage        godo.StorageService
	storageActions godo.StorageActionsService
	droplets       godo.DropletsService
	snapshots      godo.SnapshotsService
	account        godo.AccountService

	// ready defines whether the driver is ready to function. This value will
	// be used by the `Identity` service via the `Probe()` method.
	readyMu sync.Mutex // protects ready
	ready   bool
}

// NewDriver returns a CSI plugin that contains the necessary gRPC
// interfaces to interact with Kubernetes over unix domain sockets for
// managaing DigitalOcean Block Storage
func NewDriver(ep, token, url string) (*Driver, error) {
	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: token,
	})
	oauthClient := oauth2.NewClient(context.Background(), tokenSource)

	all, err := metadata.NewClient().Metadata()
	if err != nil {
		return nil, fmt.Errorf("couldn't get metadata: %s", err)
	}

	region := all.Region
	nodeId := strconv.Itoa(all.DropletID)

	opts := []godo.ClientOpt{}
	opts = append(opts, godo.SetBaseURL(url))

	doClient, err := godo.New(oauthClient, opts...)
	if err != nil {
		return nil, fmt.Errorf("couldn't initialize DigitalOcean client: %s", err)
	}

	log := logrus.New().WithFields(logrus.Fields{
		"region":  region,
		"node_id": nodeId,
		"version": version,
	})

	return &Driver{
		endpoint: ep,
		nodeId:   nodeId,
		region:   region,
		mounter:  newMounter(log),
		log:      log,

		storage:        doClient.Storage,
		storageActions: doClient.StorageActions,
		droplets:       doClient.Droplets,
		snapshots:      doClient.Snapshots,
		account:        doClient.Account,
	}, nil
}

// Run starts the CSI plugin by communication over the given endpoint
func (d *Driver) Run() error {
	u, err := url.Parse(d.endpoint)
	if err != nil {
		return fmt.Errorf("unable to parse address: %q", err)
	}

	addr := path.Join(u.Host, filepath.FromSlash(u.Path))
	if u.Host == "" {
		addr = filepath.FromSlash(u.Path)
	}

	// CSI plugins talk only over UNIX sockets currently
	if u.Scheme != "unix" {
		return fmt.Errorf("currently only unix domain sockets are supported, have: %s", u.Scheme)
	} else {
		// remove the socket if it's already there. This can happen if we
		// deploy a new version and the socket was created from the old running
		// plugin.
		d.log.WithField("socket", addr).Info("removing socket")
		if err := os.Remove(addr); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove unix domain socket file %s, error: %s", addr, err)
		}
	}

	listener, err := net.Listen(u.Scheme, addr)
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

	// warn the user, it'll not propagate to the user but at least we see if
	// something is wrong in the logs
	if err := d.checkLimit(context.Background()); err != nil {
		d.log.WithError(err).Warn("CSI plugin will not function correctly, please resolve volume limit")
	}

	d.srv = grpc.NewServer(grpc.UnaryInterceptor(errHandler))
	csi.RegisterIdentityServer(d.srv, d)
	csi.RegisterControllerServer(d.srv, d)
	csi.RegisterNodeServer(d.srv, d)

	d.ready = true // we're now ready to go!
	d.log.WithField("addr", addr).Info("server started")
	return d.srv.Serve(listener)
}

// Stop stops the plugin
func (d *Driver) Stop() {
	d.readyMu.Lock()
	d.ready = false
	d.readyMu.Unlock()

	d.log.Info("server stopped")
	d.srv.Stop()
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

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
	"encoding/json"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/digitalocean/godo"
	"github.com/kubernetes-csi/csi-test/pkg/sanity"
	"github.com/sirupsen/logrus"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

type storageVolumeRoot struct {
	Volume *godo.Volume `json:"volume"`
	Links  *godo.Links  `json:"links,omitempty"`
}

type storageVolumesRoot struct {
	Volumes []godo.Volume `json:"volumes"`
	Links   *godo.Links   `json:"links"`
}

type dropletRoot struct {
	Droplet *godo.Droplet `json:"droplet"`
	Links   *godo.Links   `json:"links,omitempty"`
}

func TestDriverSuite(t *testing.T) {
	socket := "/tmp/csi.sock"
	endpoint := "unix://" + socket
	if err := os.Remove(socket); err != nil && !os.IsNotExist(err) {
		t.Fatalf("failed to remove unix domain socket file %s, error: %s", socket, err)
	}

	// fake DO Server, not working yet ...
	nodeId := "987654"
	fake := &fakeAPI{
		t:       t,
		volumes: map[string]*godo.Volume{},
		droplets: map[string]*godo.Droplet{
			nodeId: &godo.Droplet{},
		},
	}

	ts := httptest.NewServer(fake)
	defer ts.Close()

	doClient := godo.NewClient(nil)
	url, _ := url.Parse(ts.URL)
	doClient.BaseURL = url

	driver := &Driver{
		endpoint: endpoint,
		nodeId:   nodeId,
		region:   "nyc3",
		doClient: doClient,
		mounter:  &fakeMounter{},
		log:      logrus.New().WithField("test_enabed", true),
	}
	defer driver.Stop()

	go driver.Run()

	mntDir, err := ioutil.TempDir("", "mnt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(mntDir)

	mntStageDir, err := ioutil.TempDir("", "mnt-stage")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(mntStageDir)

	cfg := &sanity.Config{
		StagingPath: mntStageDir,
		TargetPath:  mntDir,
		Address:     endpoint,
	}

	sanity.Test(t, cfg)
}

// fakeAPI implements a fake, cached DO API
type fakeAPI struct {
	t        *testing.T
	volumes  map[string]*godo.Volume
	droplets map[string]*godo.Droplet
}

func (f *fakeAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// don't deal with droplets for now
	if strings.HasPrefix(r.URL.Path, "/v2/droplets/") {
		// for now we only do a GET, so we assume it's a GET and don't check
		// for the method
		resp := new(dropletRoot)
		id := filepath.Base(r.URL.Path)
		droplet, ok := f.droplets[id]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
		} else {
			resp.Droplet = droplet
		}

		_ = json.NewEncoder(w).Encode(&resp)
		return
	}

	switch r.Method {
	case "GET":
		// A list call
		if strings.HasPrefix(r.URL.String(), "/v2/volumes?") {
			volumes := []godo.Volume{}
			if name := r.URL.Query().Get("name"); name != "" {
				for _, vol := range f.volumes {
					if vol.Name == name {
						volumes = append(volumes, *vol)
					}
				}
			} else {
				for _, vol := range f.volumes {
					volumes = append(volumes, *vol)
				}
			}

			resp := new(storageVolumesRoot)
			resp.Volumes = volumes

			err := json.NewEncoder(w).Encode(&resp)
			if err != nil {
				f.t.Fatal(err)
			}
			return

		} else {
			resp := new(storageVolumeRoot)
			// single volume get
			id := filepath.Base(r.URL.Path)
			vol, ok := f.volumes[id]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
			} else {
				resp.Volume = vol
			}

			_ = json.NewEncoder(w).Encode(&resp)
			return
		}

		// response with zero items
		var resp = struct {
			Volume []*godo.Volume
			Links  *godo.Links
		}{}

		err := json.NewEncoder(w).Encode(&resp)
		if err != nil {
			f.t.Fatal(err)
		}
	case "POST":
		v := new(godo.VolumeCreateRequest)
		err := json.NewDecoder(r.Body).Decode(v)
		if err != nil {
			f.t.Fatal(err)
		}

		id := randString(10)
		vol := &godo.Volume{
			ID: id,
			Region: &godo.Region{
				Slug: v.Region,
			},
			Description:   v.Description,
			Name:          v.Name,
			SizeGigaBytes: v.SizeGigaBytes,
			CreatedAt:     time.Now().UTC(),
		}

		f.volumes[id] = vol

		var resp = struct {
			Volume *godo.Volume
			Links  *godo.Links
		}{
			Volume: vol,
		}
		err = json.NewEncoder(w).Encode(&resp)
		if err != nil {
			f.t.Fatal(err)
		}
	case "DELETE":
		id := filepath.Base(r.URL.Path)
		delete(f.volumes, id)
	}
}

func randString(n int) string {
	const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

type fakeMounter struct{}

func (f *fakeMounter) Format(source string, fsType string) error {
	return nil
}

func (f *fakeMounter) Mount(source string, target string, fsType string, options ...string) error {
	return nil
}

func (f *fakeMounter) Unmount(target string) error {
	return nil
}

func (f *fakeMounter) IsFormatted(source string) (bool, error) {
	return true, nil
}
func (f *fakeMounter) IsMounted(target string) (bool, error) {
	return true, nil
}

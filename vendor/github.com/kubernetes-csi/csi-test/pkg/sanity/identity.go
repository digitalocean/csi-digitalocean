/*
Copyright 2017 Luis Pabón luis@portworx.com

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

package sanity

import (
	"regexp"

	csi "github.com/container-storage-interface/spec/lib/go/csi/v0"
	context "golang.org/x/net/context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

// TODO: Tests for GetPluginCapabilities

// TODO: Tests for Probe

var _ = Describe("GetPluginInfo [Identity Server]", func() {
	var (
		c csi.IdentityClient
	)

	BeforeEach(func() {
		c = csi.NewIdentityClient(conn)
	})

	It("should return appropriate information", func() {
		req := &csi.GetPluginInfoRequest{}
		res, err := c.GetPluginInfo(context.Background(), req)
		Expect(err).NotTo(HaveOccurred())
		Expect(res).NotTo(BeNil())

		By("verifying name size and characters")
		Expect(res.GetName()).ToNot(HaveLen(0))
		Expect(len(res.GetName())).To(BeNumerically("<=", 63))
		Expect(regexp.
			MustCompile("^[a-zA-Z][A-Za-z0-9-\\.\\_]{0,61}[a-zA-Z]$").
			MatchString(res.GetName())).To(BeTrue())
	})
})

package driver

import (
	"context"

	csi "github.com/container-storage-interface/spec/lib/go/csi/v0"
)

const (
	driverName    = "com.digitalocean.csi.dobs"
	vendorVersion = "0.0.1"
)

// GetPluginInfo returns metadata of the plugin
func (d *Driver) GetPluginInfo(ctx context.Context, req *csi.GetPluginInfoRequest) (*csi.GetPluginInfoResponse, error) {
	resp := &csi.GetPluginInfoResponse{
		Name:          driverName,
		VendorVersion: vendorVersion,
	}

	d.log.WithField("response", resp).Info("get plugin info called")
	return resp, nil
}

// GetPluginCapabilities returns available capabilities of the plugin
func (d *Driver) GetPluginCapabilities(ctx context.Context, req *csi.GetPluginCapabilitiesRequest) (*csi.GetPluginCapabilitiesResponse, error) {
	resp := &csi.GetPluginCapabilitiesResponse{
		Capabilities: []*csi.PluginCapability{
			{
				Type: &csi.PluginCapability_Service_{
					Service: &csi.PluginCapability_Service{
						Type: csi.PluginCapability_Service_CONTROLLER_SERVICE,
					},
				},
			},
		},
	}

	d.log.WithField("response", resp).Info("get plugin capabitilies called")
	return resp, nil
}

// Probe returns the health and readiness of the plugin
func (d *Driver) Probe(ctx context.Context, req *csi.ProbeRequest) (*csi.ProbeResponse, error) {
	d.log.Info("probe called")
	return &csi.ProbeResponse{}, nil
}

package controller

import (
	"context"
	"log/slog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"meoe.io/poly-csi/pkg/controller/dispatcher"

	"github.com/container-storage-interface/spec/lib/go/csi"
)

func CreateController(logger *slog.Logger, dispatcher *dispatcher.Dispatcher, name string, version string) (*Controller, error) {
	return &Controller{
		dispatcher: dispatcher,
		name:       name,
		version:    version,
	}, nil
}

type Controller struct {
	csi.UnimplementedIdentityServer
	csi.UnimplementedControllerServer
	// csi.UnimplementedGroupControllerServer

	name       string
	version    string
	dispatcher *dispatcher.Dispatcher
}

func (c *Controller) GetPluginInfo(ctx context.Context, req *csi.GetPluginInfoRequest) (*csi.GetPluginInfoResponse, error) {
	return &csi.GetPluginInfoResponse{
		Name:          c.name,
		VendorVersion: c.version,
	}, nil
}

func (c *Controller) Probe(ctx context.Context, req *csi.ProbeRequest) (*csi.ProbeResponse, error) {
	return &csi.ProbeResponse{
		Ready: wrapperspb.Bool(true),
	}, nil
}

func (c *Controller) GetPluginCapabilities(ctx context.Context, req *csi.GetPluginCapabilitiesRequest) (*csi.GetPluginCapabilitiesResponse, error) {
	caps := []*csi.PluginCapability{
		{
			Type: &csi.PluginCapability_Service_{
				Service: &csi.PluginCapability_Service{
					Type: csi.PluginCapability_Service_CONTROLLER_SERVICE,
				},
			},
		},
		{
			Type: &csi.PluginCapability_VolumeExpansion_{
				VolumeExpansion: &csi.PluginCapability_VolumeExpansion{
					Type: csi.PluginCapability_VolumeExpansion_ONLINE,
				},
			},
		},
		// {
		// 	Type: &csi.PluginCapability_Service_{
		// 		Service: &csi.PluginCapability_Service{
		// 			Type: csi.PluginCapability_Service_GROUP_CONTROLLER_SERVICE,
		// 		},
		// 	},
		// },
		// {
		// 		Type: &csi.PluginCapability_Service_{
		// 			Service: &csi.PluginCapability_Service{
		// 				Type: csi.PluginCapability_Service_VOLUME_ACCESSIBILITY_CONSTRAINTS,
		// 			},
		// 		},
		// 	}
		// {
		// 		Type: &csi.PluginCapability_Service_{
		// 			Service: &csi.PluginCapability_Service{
		// 				Type: csi.PluginCapability_Service_VOLUME_ACCESSIBILITY_CONSTRAINTS,
		// 			},
		// 		},
		// 	}
	}

	return &csi.GetPluginCapabilitiesResponse{Capabilities: caps}, nil
}

func (c *Controller) ControllerGetCapabilities(ctx context.Context, req *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	cl := []csi.ControllerServiceCapability_RPC_Type{
		csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
		csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME,
		// csi.ControllerServiceCapability_RPC_LIST_VOLUMES,
		// csi.ControllerServiceCapability_RPC_GET_CAPACITY,
		// csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT,
		// csi.ControllerServiceCapability_RPC_LIST_SNAPSHOTS,
		csi.ControllerServiceCapability_RPC_CLONE_VOLUME,
		// csi.ControllerServiceCapability_RPC_PUBLISH_READONLY,
		csi.ControllerServiceCapability_RPC_EXPAND_VOLUME,
		// csi.ControllerServiceCapability_RPC_LIST_VOLUMES_PUBLISHED_NODES,
		csi.ControllerServiceCapability_RPC_GET_VOLUME,
		csi.ControllerServiceCapability_RPC_SINGLE_NODE_MULTI_WRITER,
		csi.ControllerServiceCapability_RPC_MODIFY_VOLUME,
		// csi.ControllerServiceCapability_RPC_GET_SNAPSHOT,
		csi.ControllerServiceCapability_RPC_GET_VOLUME_HEALTH,
		// csi.ControllerServiceCapability_RPC_LIST_VOLUME_HEALTH,
	}

	var csc []*csi.ControllerServiceCapability

	for _, cap := range cl {
		csc = append(csc, &csi.ControllerServiceCapability{
			Type: &csi.ControllerServiceCapability_Rpc{
				Rpc: &csi.ControllerServiceCapability_RPC{
					Type: cap,
				},
			},
		})
	}

	return &csi.ControllerGetCapabilitiesResponse{
		Capabilities: csc,
	}, nil
}

func (c *Controller) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (resp *csi.CreateVolumeResponse, finalErr error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "request name is empty")
	}
	if req.CapacityRange == nil {
		return nil, status.Error(codes.InvalidArgument, "capacity spec is missing")
	}
	if req.VolumeCapabilities == nil {
		return nil, status.Error(codes.InvalidArgument, "volume capabilities missing")
	}
	var accessTypeMount, accessTypeBlock bool
	for _, cap := range req.VolumeCapabilities {
		if cap.GetBlock() != nil {
			accessTypeBlock = true
		}
		if cap.GetMount() != nil {
			accessTypeMount = true
		}
	}
	if accessTypeBlock && accessTypeMount {
		return nil, status.Error(codes.InvalidArgument, "conflicting mount type: block and mount")
	}

	resp, err := c.dispatcher.CreateVolume(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (c *Controller) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "volume id is empty")
	}

	resp, err := c.dispatcher.DeleteVolume(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (c *Controller) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "volume id is empty")
	}
	if len(req.VolumeCapabilities) == 0 {
		return nil, status.Error(codes.InvalidArgument, "empty volume capabilities")
	}

	return &csi.ValidateVolumeCapabilitiesResponse{
		Confirmed: &csi.ValidateVolumeCapabilitiesResponse_Confirmed{
			VolumeContext:      req.GetVolumeContext(),
			VolumeCapabilities: req.GetVolumeCapabilities(),
			Parameters:         req.GetParameters(),
		},
	}, nil
}

func (c *Controller) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "volume id is empty")
	}
	if req.NodeId == "" {
		return nil, status.Error(codes.InvalidArgument, "node id is empty")
	}
	if req.VolumeCapability == nil {
		return nil, status.Error(codes.InvalidArgument, "volume capabilities missing")
	}

	return c.dispatcher.ControllerPublishVolume(ctx, req)
}

func (c *Controller) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "volume id is empty")
	}
	// empty node_id is allowed here

	return c.dispatcher.ControllerUnpublishVolume(ctx, req)
}

func (c *Controller) GetCapacity(ctx context.Context, req *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
	return c.dispatcher.GetCapacity(ctx, req)
}

func (c *Controller) ListVolumes(ctx context.Context, req *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	return c.dispatcher.ListVolumes(ctx, req)
}

func (c *Controller) ControllerGetVolume(ctx context.Context, req *csi.ControllerGetVolumeRequest) (*csi.ControllerGetVolumeResponse, error) {
	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "volume id is empty")
	}
	return c.dispatcher.ControllerGetVolume(ctx, req)
}

func (c *Controller) ControllerModifyVolume(ctx context.Context, req *csi.ControllerModifyVolumeRequest) (*csi.ControllerModifyVolumeResponse, error) {
	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "volume id is empty")
	}
	if len(req.MutableParameters) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Mutable parameters cannot be empty")
	}

	return c.dispatcher.ControllerModifyVolume(ctx, req)
}

func (c *Controller) ControllerExpandVolume(ctx context.Context, req *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "volume id is empty")
	}
	if req.CapacityRange == nil {
		return nil, status.Error(codes.InvalidArgument, "capacity spec is missing")
	}

	return c.dispatcher.ControllerExpandVolume(ctx, req)
}

func (c *Controller) ControllerGetVolumeHealth(ctx context.Context, req *csi.ControllerGetVolumeHealthRequest) (*csi.ControllerGetVolumeHealthResponse, error) {
	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "volume id is empty")
	}

	return c.dispatcher.ControllerGetVolumeHealth(ctx, req)
}

func (c *Controller) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "request name is empty")
	}
	if req.SourceVolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "source volume id is empty")
	}

	return c.dispatcher.CreateSnapshot(ctx, req)
}

func (c *Controller) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	if req.SnapshotId == "" {
		return nil, status.Error(codes.InvalidArgument, "snapshot id is empty")
	}

	return c.dispatcher.DeleteSnapshot(ctx, req)
}

func (c *Controller) GetConfig() *dispatcher.DispatcherConf {
	return c.dispatcher.GetConfig()
}

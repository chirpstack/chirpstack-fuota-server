package api

import (
	"context"

	"github.com/gofrs/uuid"
	"github.com/golang/protobuf/ptypes"
	log "github.com/sirupsen/logrus"

	"github.com/brocaar/lorawan"
	fapi "github.com/chirpstack/chirpstack-fuota-server/v4/api/go"
	"github.com/chirpstack/chirpstack-fuota-server/v4/internal/fuota"
	"github.com/chirpstack/chirpstack-fuota-server/v4/internal/storage"
	"github.com/chirpstack/chirpstack/api/go/v4/api"
)

// FUOTAServerAPI implements the FUOTA server API.
type FUOTAServerAPI struct {
	fapi.UnimplementedFuotaServerServiceServer
}

// NewFUOTAServerAPI creates a new FUOTAServerAPI.
func NewFUOTAServerAPI() *FUOTAServerAPI {
	return &FUOTAServerAPI{}
}

// CreateDeployment creates the given FUOTA deployment.
func (a *FUOTAServerAPI) CreateDeployment(ctx context.Context, req *fapi.CreateDeploymentRequest) (*fapi.CreateDeploymentResponse, error) {
	opts := fuota.DeploymentOptions{
		ApplicationID:                     req.GetDeployment().ApplicationId,
		Devices:                           make(map[lorawan.EUI64]fuota.DeviceOptions),
		MulticastDR:                       uint8(req.GetDeployment().MulticastDr),
		MulticastFrequency:                req.GetDeployment().MulticastFrequency,
		MulticastGroupID:                  uint8(req.GetDeployment().MulticastGroupId),
		MulticastTimeout:                  uint8(req.GetDeployment().MulticastTimeout),
		FragSize:                          int(req.GetDeployment().FragmentationFragmentSize),
		Payload:                           req.GetDeployment().Payload,
		Redundancy:                        int(req.GetDeployment().FragmentationRedundancy),
		FragmentationSessionIndex:         uint8(req.GetDeployment().FragmentationSessionIndex),
		FragmentationMatrix:               uint8(req.GetDeployment().FragmentationMatrix),
		BlockAckDelay:                     uint8(req.GetDeployment().FragmentationBlockAckDelay),
		UnicastAttemptCount:               int(req.GetDeployment().UnicastAttemptCount),
		RequestFragmentationSessionStatus: fuota.FragmentationSessionStatusRequestType(req.GetDeployment().RequestFragmentationSessionStatus.String()),
	}

	for _, d := range req.GetDeployment().Devices {
		var devEUI lorawan.EUI64
		if err := devEUI.UnmarshalText([]byte(d.DevEui)); err != nil {
			return nil, err
		}

		var mcRootKey lorawan.AES128Key
		if err := mcRootKey.UnmarshalText([]byte(d.McRootKey)); err != nil {
			return nil, err
		}

		opts.Devices[devEUI] = fuota.DeviceOptions{
			McRootKey: mcRootKey,
		}
	}

	switch req.GetDeployment().MulticastGroupType {
	case fapi.MulticastGroupType_CLASS_B:
		opts.MulticastGroupType = api.MulticastGroupType_CLASS_B
	case fapi.MulticastGroupType_CLASS_C:
		opts.MulticastGroupType = api.MulticastGroupType_CLASS_C
	}

	copy(opts.Descriptor[:], req.GetDeployment().FragmentationDescriptor)

	unicastTimeout, err := ptypes.Duration(req.GetDeployment().UnicastTimeout)
	if err != nil {
		return nil, err
	}

	opts.UnicastTimeout = unicastTimeout

	depl, err := fuota.NewDeployment(opts)
	if err != nil {
		return nil, err
	}

	go func(depl *fuota.Deployment) {
		if err := depl.Run(context.Background()); err != nil {
			log.WithError(err).WithField("deployment_id", depl.GetID()).Error("api: fuota deployment error")
		}
	}(depl)

	return &fapi.CreateDeploymentResponse{
		Id: depl.GetID().String(),
	}, nil
}

// GetDeploymentStatus returns the FUOTA deployment status given an ID.
func (a *FUOTAServerAPI) GetDeploymentStatus(ctx context.Context, req *fapi.GetDeploymentStatusRequest) (*fapi.GetDeploymentStatusResponse, error) {
	id, err := uuid.FromString(req.GetId())
	if err != nil {
		return nil, err
	}

	d, err := storage.GetDeployment(ctx, storage.DB(), id)
	if err != nil {
		return nil, err
	}

	var resp fapi.GetDeploymentStatusResponse

	resp.CreatedAt, err = ptypes.TimestampProto(d.CreatedAt)
	if err != nil {
		return nil, err
	}

	resp.UpdatedAt, err = ptypes.TimestampProto(d.UpdatedAt)
	if err != nil {
		return nil, err
	}

	if d.MCGroupSetupCompletedAt != nil {
		resp.McGroupSetupCompletedAt, err = ptypes.TimestampProto(*d.MCGroupSetupCompletedAt)
		if err != nil {
			return nil, err
		}
	}

	if d.MCSessionCompletedAt != nil {
		resp.McSessionCompletedAt, err = ptypes.TimestampProto(*d.MCSessionCompletedAt)
		if err != nil {
			return nil, err
		}
	}

	if d.FragSessionSetupCompletedAt != nil {
		resp.FragSessionSetupCompletedAt, err = ptypes.TimestampProto(*d.FragSessionSetupCompletedAt)
		if err != nil {
			return nil, err
		}
	}

	if d.EnqueueCompletedAt != nil {
		resp.EnqueueCompletedAt, err = ptypes.TimestampProto(*d.EnqueueCompletedAt)
		if err != nil {
			return nil, err
		}
	}

	if d.FragStatusCompletedAt != nil {
		resp.FragStatusCompletedAt, err = ptypes.TimestampProto(*d.FragStatusCompletedAt)
		if err != nil {
			return nil, err
		}
	}

	devices, err := storage.GetDeploymentDevices(ctx, storage.DB(), id)
	if err != nil {
		return nil, err
	}

	for _, device := range devices {
		dd := fapi.DeploymentDeviceStatus{
			DevEui: device.DevEUI.String(),
		}
		var err error

		dd.CreatedAt, err = ptypes.TimestampProto(device.CreatedAt)
		if err != nil {
			return nil, err
		}

		dd.UpdatedAt, err = ptypes.TimestampProto(device.UpdatedAt)
		if err != nil {
			return nil, err
		}

		if device.MCGroupSetupCompletedAt != nil {
			dd.McGroupSetupCompletedAt, err = ptypes.TimestampProto(*device.MCGroupSetupCompletedAt)
			if err != nil {
				return nil, err
			}
		}

		if device.MCSessionCompletedAt != nil {
			dd.McSessionCompletedAt, err = ptypes.TimestampProto(*device.MCSessionCompletedAt)
			if err != nil {
				return nil, err
			}
		}

		if device.FragSessionSetupCompletedAt != nil {
			dd.FragSessionSetupCompletedAt, err = ptypes.TimestampProto(*device.FragSessionSetupCompletedAt)
			if err != nil {
				return nil, err
			}
		}

		if device.FragStatusCompletedAt != nil {
			dd.FragStatusCompletedAt, err = ptypes.TimestampProto(*device.FragStatusCompletedAt)
			if err != nil {
				return nil, err
			}
		}

		resp.DeviceStatus = append(resp.DeviceStatus, &dd)
	}

	return &resp, nil
}

// GetDeploymentDeviceLogs returns the FUOTA logs given a deployment ID and DevEUI.
func (a *FUOTAServerAPI) GetDeploymentDeviceLogs(ctx context.Context, req *fapi.GetDeploymentDeviceLogsRequest) (*fapi.GetDeploymentDeviceLogsResponse, error) {
	var devEUI lorawan.EUI64
	var resp fapi.GetDeploymentDeviceLogsResponse

	deploymentID, err := uuid.FromString(req.GetDeploymentId())
	if err != nil {
		return nil, err
	}

	if err := devEUI.UnmarshalText([]byte(req.GetDevEui())); err != nil {
		return nil, err
	}

	logs, err := storage.GetDeploymentLogsForDevice(ctx, storage.DB(), deploymentID, devEUI)
	if err != nil {
		return nil, err
	}

	for _, l := range logs {
		dl := fapi.DeploymentDeviceLog{
			FPort:   uint32(l.FPort),
			Command: l.Command,
			Fields:  make(map[string]string),
		}

		dl.CreatedAt, err = ptypes.TimestampProto(l.CreatedAt)
		if err != nil {
			return nil, err
		}

		for k, v := range l.Fields.Map {
			dl.Fields[k] = v.String
		}

		resp.Logs = append(resp.Logs, &dl)
	}

	return &resp, nil
}

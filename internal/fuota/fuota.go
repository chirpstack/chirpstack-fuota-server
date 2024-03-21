package fuota

import (
	"context"
	"crypto/aes"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/gofrs/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq/hstore"
	log "github.com/sirupsen/logrus"

	"github.com/brocaar/lorawan"
	"github.com/brocaar/lorawan/applayer/fragmentation"
	"github.com/brocaar/lorawan/applayer/multicastsetup"
	"github.com/brocaar/lorawan/gps"
	"github.com/chirpstack/chirpstack-fuota-server/v4/internal/client/as"
	"github.com/chirpstack/chirpstack-fuota-server/v4/internal/eventhandler"
	"github.com/chirpstack/chirpstack-fuota-server/v4/internal/storage"
	"github.com/chirpstack/chirpstack/api/go/v4/api"
	"github.com/chirpstack/chirpstack/api/go/v4/common"
	"github.com/chirpstack/chirpstack/api/go/v4/integration"
)

// FragmentationSessionStatusRequestType type.
type FragmentationSessionStatusRequestType string

// RequestFragmentationSessionStatus options.
const (
	RequestFragmentationSessionStatusAfterFragmentEnqueue FragmentationSessionStatusRequestType = "AFTER_FRAGMENT_ENQUEUE"
	RequestFragmentationSessionStatusAfterSessionTimeout  FragmentationSessionStatusRequestType = "AFTER_SESSION_TIMEOUT"
	RequestFragmentationSessionStatusNoRequest            FragmentationSessionStatusRequestType = "NO_REQUEST"
)

// Deployments defines the FUOTA deployment struct.
type Deployment struct {
	opts DeploymentOptions
	id   uuid.UUID

	// this contains the ID from CreateMulticastGroupResponse
	multicastGroupID string

	// McAddr.
	mcAddr lorawan.DevAddr

	// mcKey.
	mcKey lorawan.AES128Key

	// deviceState contains the per device FUOTA state
	deviceState map[lorawan.EUI64]*deviceState

	// channel for completing the multicast-setup
	multicastSetupDone chan struct{}

	// channel for fragmentation-session setup
	fragmentationSessionSetupDone chan struct{}

	// channel multicast session setup
	multicastSessionSetupDone chan struct{}

	// channel for fragmentation-session status
	fragmentationSessionStatusDone chan struct{}

	// session start time
	// this is set by the multicast-session setup function
	sessionStartTime time.Time

	// session end time
	// this is set by the multicast-session setup function
	sessionEndTime time.Time
}

// DeploymentOptions defines the options for a FUOTA Deployment.
type DeploymentOptions struct {
	// The application id.
	ApplicationID string

	// The Devices to include in the update.
	// Note: the devices must be part of the above application id.
	Devices map[lorawan.EUI64]DeviceOptions

	// MulticastGroupType defines the multicast type (B/C)
	MulticastGroupType api.MulticastGroupType

	// Multicast DR defines the multicast data-rate.
	MulticastDR uint8

	// MulticastPingSlotPeriodicity defines the ping-slot periodicity (Class-B).
	// Expected values: 0 -7.
	MulticastPingSlotPeriodicity uint8

	// MulticastFrequency defines the frequency.
	MulticastFrequency uint32

	// MulticastGroupID defines the multicast group ID.
	MulticastGroupID uint8

	// MulticastTimeout defines the timeout of the multicast-session.
	// Please refer to the Remote Multicast Setup specification as this field
	// has a different meaning for Class-B and Class-C groups.
	MulticastTimeout uint8

	// MulticastRegion defines the multicast region.
	MulticastRegion common.Region

	// UnicastTimeout.
	// Set this to the value in which you at least expect an uplink frame from
	// the device. The FUOTA server will wait for the given time before
	// attempting a retry or continuing with the next step.
	UnicastTimeout time.Duration

	// UnicastAttemptCount.
	// The number of attempts before considering an unicast command
	// to be failed.
	UnicastAttemptCount int

	// FragSize defines the max size for each payload fragment.
	FragSize int

	// Payload defines the FUOTA payload.
	Payload []byte

	// Redundancy (in number of packets).
	Redundancy int

	// FragmentationSessionIndex.
	FragmentationSessionIndex uint8

	// FragmentationMatrix.
	FragmentationMatrix uint8

	// BlockAckDelay.
	BlockAckDelay uint8

	// Descriptor.
	Descriptor [4]byte

	// RequestFragmentationSessionStatus defines if and when the frag-session
	// status must be requested.
	RequestFragmentationSessionStatus FragmentationSessionStatusRequestType
}

// DeviceOptions holds the device options.
type DeviceOptions struct {
	// McRootKey holds the McRootKey.
	// Note: please refer to the Remote Multicast Setup specification for more
	// information (page 10).
	McRootKey lorawan.AES128Key
}

// deviceState contains the FUOTA state of a device
type deviceState struct {
	sync.RWMutex
	multicastSetup             bool
	fragmentationSessionSetup  bool
	multicastSessionSetup      bool
	fragmentationSessionStatus bool
}

func (d *deviceState) getMulticastSetup() bool {
	d.RLock()
	defer d.RUnlock()
	return d.multicastSetup
}

func (d *deviceState) setMulticastSetup(done bool) {
	d.Lock()
	defer d.Unlock()
	d.multicastSetup = done
}

func (d *deviceState) getFragmentationSessionSetup() bool {
	d.RLock()
	defer d.RUnlock()
	return d.fragmentationSessionSetup
}

func (d *deviceState) setFragmentationSessionSetup(done bool) {
	d.Lock()
	defer d.Unlock()
	d.fragmentationSessionSetup = done
}

func (d *deviceState) setMulicastSessionSetup(done bool) {
	d.Lock()
	defer d.Unlock()
	d.multicastSessionSetup = done
}

func (d *deviceState) getMulticastSessionSetup() bool {
	d.RLock()
	defer d.RUnlock()
	return d.multicastSessionSetup
}

func (d *deviceState) setFragmentationSessionStatus(done bool) {
	d.Lock()
	defer d.Unlock()
	d.fragmentationSessionStatus = done
}

func (d *deviceState) getFragmentationSessionStatus() bool {
	d.RLock()
	defer d.RUnlock()
	return d.fragmentationSessionStatus
}

// NewDeployment creates a new Deployment.
func NewDeployment(opts DeploymentOptions) (*Deployment, error) {
	id, err := uuid.NewV4()
	if err != nil {
		return nil, fmt.Errorf("new uuid error: %w", err)
	}

	d := Deployment{
		id:          id,
		opts:        opts,
		deviceState: make(map[lorawan.EUI64]*deviceState),

		multicastSetupDone:             make(chan struct{}),
		fragmentationSessionSetupDone:  make(chan struct{}),
		multicastSessionSetupDone:      make(chan struct{}),
		fragmentationSessionStatusDone: make(chan struct{}),
	}

	if err := storage.Transaction(func(tx sqlx.Ext) error {
		st := storage.Deployment{
			ID: d.id,
		}
		if err := storage.CreateDeployment(context.Background(), tx, &st); err != nil {
			return fmt.Errorf("create deployment error: %w", err)
		}

		for devEUI, _ := range opts.Devices {
			d.deviceState[devEUI] = &deviceState{}

			sdd := storage.DeploymentDevice{
				DeploymentID: d.id,
				DevEUI:       devEUI,
			}
			if err := storage.CreateDeploymentDevice(context.Background(), tx, &sdd); err != nil {
				return fmt.Errorf("create deployment device error: %w", err)
			}
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return &d, nil
}

// GetID returns the random assigned FUOTA deployment ID.
func (d *Deployment) GetID() uuid.UUID {
	return d.id
}

// Run starts the FUOTA update.
func (d *Deployment) Run(ctx context.Context) error {
	eventhandler.Get().RegisterUplinkEventFunc(d.GetID(), d.HandleUplinkEvent)
	defer eventhandler.Get().UnregisterUplinkEventFunc(d.GetID())

	steps := []func(context.Context) error{
		d.stepCreateMulticastGroup,
		d.stepAddDevicesToMulticastGroup,
		d.stepMulticastSetup,
		d.stepFragmentationSessionSetup,
		d.stepMulticastClassBSessionSetup,
		d.stepMulticastClassCSessionSetup,
		d.stepEnqueue,
		d.stepFragSessionStatus,
		d.stepWaitUntilTimeout,
		d.stepDeleteMulticastGroup,
	}

	for _, f := range steps {
		if err := f(ctx); err != nil {
			return err
		}
	}

	return nil
}

// HandleUplinkEvent handles the given uplink event.
// In case it does not match one of the FUOTA ports or DevEUI within the
// deployment, the uplink is silently discarded.
func (d *Deployment) HandleUplinkEvent(ctx context.Context, pl integration.UplinkEvent) error {
	var devEUI lorawan.EUI64
	if err := devEUI.UnmarshalText([]byte(pl.GetDeviceInfo().GetDevEui())); err != nil {
		return err
	}
	_, found := d.opts.Devices[devEUI]

	if uint8(pl.FPort) == multicastsetup.DefaultFPort && found {
		if err := d.handleMulticastSetupCommand(ctx, devEUI, pl.Data); err != nil {
			return fmt.Errorf("handle multicast setup command error: %w", err)
		}
	} else if uint8(pl.FPort) == fragmentation.DefaultFPort && found {
		if err := d.handleFragmentationSessionSetupCommand(ctx, devEUI, pl.Data); err != nil {
			return fmt.Errorf("handle fragmentation-session setup command error: %w", err)
		}
	} else {
		log.WithFields(log.Fields{
			"deployment_id": d.id,
			"dev_eui":       devEUI,
			"f_port":        pl.FPort,
		}).Debug("fuota: ignoring uplink event")
	}

	return nil
}

// handleMulticastSetupCommand handles an uplink multicast setup command.
func (d *Deployment) handleMulticastSetupCommand(ctx context.Context, devEUI lorawan.EUI64, b []byte) error {
	var cmd multicastsetup.Command
	if err := cmd.UnmarshalBinary(true, b); err != nil {
		return fmt.Errorf("unmarshal command error: %w", err)
	}

	log.WithFields(log.Fields{
		"deployment_id": d.GetID(),
		"dev_eui":       devEUI,
		"cid":           cmd.CID,
	}).Info("fuota: multicast-setup command received")

	switch cmd.CID {
	case multicastsetup.McGroupSetupAns:
		pl, ok := cmd.Payload.(*multicastsetup.McGroupSetupAnsPayload)
		if !ok {
			return fmt.Errorf("expected *multicastsetup.McGroupSetupAnsPayload, got: %T", cmd.Payload)
		}
		return d.handleMcGroupSetupAns(ctx, devEUI, pl)
	case multicastsetup.McClassBSessionAns:
		pl, ok := cmd.Payload.(*multicastsetup.McClassBSessionAnsPayload)
		if !ok {
			return fmt.Errorf("expected *multicastsetup.McClassBSessionAnsPayload, got: %T", cmd.Payload)
		}
		return d.handleMcClassBSessionAns(ctx, devEUI, pl)
	case multicastsetup.McClassCSessionAns:
		pl, ok := cmd.Payload.(*multicastsetup.McClassCSessionAnsPayload)
		if !ok {
			return fmt.Errorf("expected *multicastsetup.McClassCSessionAnsPayload, got: %T", cmd.Payload)
		}
		return d.handleMcClassCSessionAns(ctx, devEUI, pl)
	}

	return nil
}

// handleFragmentationSessionSetupCommand handles an uplink fragmentation-session setup command.
func (d *Deployment) handleFragmentationSessionSetupCommand(ctx context.Context, devEUI lorawan.EUI64, b []byte) error {
	var cmd fragmentation.Command
	if err := cmd.UnmarshalBinary(true, b); err != nil {
		return fmt.Errorf("unmarshal command error: %w", err)
	}

	log.WithFields(log.Fields{
		"deployment_id": d.GetID(),
		"dev_eui":       devEUI,
		"cid":           cmd.CID,
	}).Info("fuota: fragmentation-session setup command received")

	switch cmd.CID {
	case fragmentation.FragSessionSetupAns:
		pl, ok := cmd.Payload.(*fragmentation.FragSessionSetupAnsPayload)
		if !ok {
			return fmt.Errorf("expected *fragmentation.FragSessionSetupAnsPayload, got: %T", cmd.Payload)
		}
		return d.handleFragSessionSetupAns(ctx, devEUI, pl)
	case fragmentation.FragSessionStatusAns:
		pl, ok := cmd.Payload.(*fragmentation.FragSessionStatusAnsPayload)
		if !ok {
			return fmt.Errorf("expected *fragmentation.FragSessionStatusAnsPayload, got: %T", cmd.Payload)
		}
		return d.handleFragSessionStatusAns(ctx, devEUI, pl)
	}

	return nil
}

func (d *Deployment) handleMcGroupSetupAns(ctx context.Context, devEUI lorawan.EUI64, pl *multicastsetup.McGroupSetupAnsPayload) error {
	log.WithFields(log.Fields{
		"deployment_id": d.GetID(),
		"dev_eui":       devEUI,
		"id_error":      pl.McGroupIDHeader.IDError,
		"mc_group_id":   pl.McGroupIDHeader.McGroupID,
	}).Info("fuota: McGroupSetupAns received")

	dl := storage.DeploymentLog{
		DeploymentID: d.GetID(),
		DevEUI:       devEUI,
		FPort:        multicastsetup.DefaultFPort,
		Command:      "McGroupSetupAns",
		Fields: hstore.Hstore{
			Map: map[string]sql.NullString{
				"mc_group_id": sql.NullString{Valid: true, String: fmt.Sprintf("%d", pl.McGroupIDHeader.McGroupID)},
				"id_error":    sql.NullString{Valid: true, String: fmt.Sprintf("%t", pl.McGroupIDHeader.IDError)},
			},
		},
	}
	if err := storage.CreateDeploymentLog(ctx, storage.DB(), &dl); err != nil {
		log.WithError(err).Error("fuota: create deployment log error")
	}

	if pl.McGroupIDHeader.McGroupID == d.opts.MulticastGroupID && !pl.McGroupIDHeader.IDError {
		// update the device state
		if state, ok := d.deviceState[devEUI]; ok {
			state.setMulticastSetup(true)
		}

		dd, err := storage.GetDeploymentDevice(ctx, storage.DB(), d.GetID(), devEUI)
		if err != nil {
			return fmt.Errorf("get deployment device error: %w", err)
		}
		now := time.Now()
		dd.MCGroupSetupCompletedAt = &now
		if err := storage.UpdateDeploymentDevice(ctx, storage.DB(), &dd); err != nil {
			return fmt.Errorf("update deployment device error: %w", err)
		}

		// if all devices have finished the multicast-setup, publish to done chan.
		done := true
		for _, state := range d.deviceState {
			if !state.getMulticastSetup() {
				done = false
			}
		}
		if done {
			d.multicastSetupDone <- struct{}{}
		}
	}

	return nil
}

func (d *Deployment) handleFragSessionSetupAns(ctx context.Context, devEUI lorawan.EUI64, pl *fragmentation.FragSessionSetupAnsPayload) error {
	log.WithFields(log.Fields{
		"deployment_id":                    d.GetID(),
		"dev_eui":                          devEUI,
		"frag_index":                       pl.StatusBitMask.FragIndex,
		"wrong_descriptor":                 pl.StatusBitMask.WrongDescriptor,
		"frag_session_index_not_supported": pl.StatusBitMask.FragSessionIndexNotSupported,
		"not_enough_memory":                pl.StatusBitMask.NotEnoughMemory,
		"encoding_unsupported":             pl.StatusBitMask.EncodingUnsupported,
	}).Info("fuota: FragSessionSetupAns received")

	dl := storage.DeploymentLog{
		DeploymentID: d.GetID(),
		DevEUI:       devEUI,
		FPort:        fragmentation.DefaultFPort,
		Command:      "FragSessionSetupAns",
		Fields: hstore.Hstore{
			Map: map[string]sql.NullString{
				"frag_index":                       sql.NullString{Valid: true, String: fmt.Sprintf("%d", pl.StatusBitMask.FragIndex)},
				"wrong_descriptor":                 sql.NullString{Valid: true, String: fmt.Sprintf("%t", pl.StatusBitMask.WrongDescriptor)},
				"frag_session_index_not_supported": sql.NullString{Valid: true, String: fmt.Sprintf("%t", pl.StatusBitMask.FragSessionIndexNotSupported)},
				"not_enough_memory":                sql.NullString{Valid: true, String: fmt.Sprintf("%t", pl.StatusBitMask.NotEnoughMemory)},
				"encoding_unsupported":             sql.NullString{Valid: true, String: fmt.Sprintf("%t", pl.StatusBitMask.EncodingUnsupported)},
			},
		},
	}
	if err := storage.CreateDeploymentLog(ctx, storage.DB(), &dl); err != nil {
		log.WithError(err).Error("fuota: create deployment log error")
	}

	if pl.StatusBitMask.FragIndex == d.opts.FragmentationSessionIndex && (!pl.StatusBitMask.WrongDescriptor && !pl.StatusBitMask.FragSessionIndexNotSupported && !pl.StatusBitMask.NotEnoughMemory && !pl.StatusBitMask.EncodingUnsupported) {
		// update the device state
		if state, ok := d.deviceState[devEUI]; ok {
			state.setFragmentationSessionSetup(true)
		}

		dd, err := storage.GetDeploymentDevice(ctx, storage.DB(), d.GetID(), devEUI)
		if err != nil {
			return fmt.Errorf("get deployment device error: %w", err)
		}
		now := time.Now()
		dd.FragSessionSetupCompletedAt = &now
		if err := storage.UpdateDeploymentDevice(ctx, storage.DB(), &dd); err != nil {
			return fmt.Errorf("update deployment device error: %w", err)
		}

		// if all devices have finished the fragmentation-session setup, publish to done chan.
		done := true
		for _, state := range d.deviceState {
			// ignore devices that have not setup multicast
			if !state.getMulticastSetup() {
				continue
			}

			if !state.getFragmentationSessionSetup() {
				done = false
			}
		}
		if done {
			d.fragmentationSessionSetupDone <- struct{}{}
		}
	}

	return nil
}

func (d *Deployment) handleMcClassBSessionAns(ctx context.Context, devEUI lorawan.EUI64, pl *multicastsetup.McClassBSessionAnsPayload) error {
	log.WithFields(log.Fields{
		"deployment_id":      d.GetID(),
		"dev_eui":            devEUI,
		"mc_group_undefined": pl.StatusAndMcGroupID.McGroupUndefined,
		"freq_error":         pl.StatusAndMcGroupID.FreqError,
		"dr_error":           pl.StatusAndMcGroupID.DRError,
		"mc_group_id":        pl.StatusAndMcGroupID.McGroupID,
	}).Info("fuota: McClassBSessionAns received")

	dl := storage.DeploymentLog{
		DeploymentID: d.GetID(),
		DevEUI:       devEUI,
		FPort:        multicastsetup.DefaultFPort,
		Command:      "McClassBSessionAns",
		Fields: hstore.Hstore{
			Map: map[string]sql.NullString{
				"mc_group_undefined": sql.NullString{Valid: true, String: fmt.Sprintf("%t", pl.StatusAndMcGroupID.McGroupUndefined)},
				"freq_error":         sql.NullString{Valid: true, String: fmt.Sprintf("%t", pl.StatusAndMcGroupID.FreqError)},
				"dr_error":           sql.NullString{Valid: true, String: fmt.Sprintf("%t", pl.StatusAndMcGroupID.DRError)},
				"mc_group_id":        sql.NullString{Valid: true, String: fmt.Sprintf("%d", pl.StatusAndMcGroupID.McGroupID)},
			},
		},
	}
	if err := storage.CreateDeploymentLog(context.Background(), storage.DB(), &dl); err != nil {
		log.WithError(err).Error("fuota: create deployment log error")
	}

	if pl.StatusAndMcGroupID.McGroupID == d.opts.MulticastGroupID && (!pl.StatusAndMcGroupID.McGroupUndefined && !pl.StatusAndMcGroupID.FreqError && !pl.StatusAndMcGroupID.DRError) {
		// update the device state
		if state, ok := d.deviceState[devEUI]; ok {
			state.setMulicastSessionSetup(true)
		}

		dd, err := storage.GetDeploymentDevice(ctx, storage.DB(), d.GetID(), devEUI)
		if err != nil {
			return fmt.Errorf("get deployment device error: %w", err)
		}
		now := time.Now()
		dd.MCSessionCompletedAt = &now
		if err := storage.UpdateDeploymentDevice(ctx, storage.DB(), &dd); err != nil {
			return fmt.Errorf("update deployment device error: %w", err)
		}

		// if all devices have finished the multicast class-c session setup, publish to done chan.
		done := true
		for _, state := range d.deviceState {
			// ignore devices that have not setup the fragmentation session
			if !state.getFragmentationSessionSetup() {
				continue
			}

			if !state.getMulticastSessionSetup() {
				done = false
			}
		}
		if done {
			d.multicastSessionSetupDone <- struct{}{}
		}
	}

	return nil
}

func (d *Deployment) handleMcClassCSessionAns(ctx context.Context, devEUI lorawan.EUI64, pl *multicastsetup.McClassCSessionAnsPayload) error {
	log.WithFields(log.Fields{
		"deployment_id":      d.GetID(),
		"dev_eui":            devEUI,
		"mc_group_undefined": pl.StatusAndMcGroupID.McGroupUndefined,
		"freq_error":         pl.StatusAndMcGroupID.FreqError,
		"dr_error":           pl.StatusAndMcGroupID.DRError,
		"mc_group_id":        pl.StatusAndMcGroupID.McGroupID,
	}).Info("fuota: McClassCSessionAns received")

	dl := storage.DeploymentLog{
		DeploymentID: d.GetID(),
		DevEUI:       devEUI,
		FPort:        multicastsetup.DefaultFPort,
		Command:      "McClassCSessionAns",
		Fields: hstore.Hstore{
			Map: map[string]sql.NullString{
				"mc_group_undefined": sql.NullString{Valid: true, String: fmt.Sprintf("%t", pl.StatusAndMcGroupID.McGroupUndefined)},
				"freq_error":         sql.NullString{Valid: true, String: fmt.Sprintf("%t", pl.StatusAndMcGroupID.FreqError)},
				"dr_error":           sql.NullString{Valid: true, String: fmt.Sprintf("%t", pl.StatusAndMcGroupID.DRError)},
				"mc_group_id":        sql.NullString{Valid: true, String: fmt.Sprintf("%d", pl.StatusAndMcGroupID.McGroupID)},
			},
		},
	}
	if err := storage.CreateDeploymentLog(context.Background(), storage.DB(), &dl); err != nil {
		log.WithError(err).Error("fuota: create deployment log error")
	}

	if pl.StatusAndMcGroupID.McGroupID == d.opts.MulticastGroupID && (!pl.StatusAndMcGroupID.McGroupUndefined && !pl.StatusAndMcGroupID.FreqError && !pl.StatusAndMcGroupID.DRError) {
		// update the device state
		if state, ok := d.deviceState[devEUI]; ok {
			state.setMulicastSessionSetup(true)
		}

		dd, err := storage.GetDeploymentDevice(ctx, storage.DB(), d.GetID(), devEUI)
		if err != nil {
			return fmt.Errorf("get deployment device error: %w", err)
		}
		now := time.Now()
		dd.MCSessionCompletedAt = &now
		if err := storage.UpdateDeploymentDevice(ctx, storage.DB(), &dd); err != nil {
			return fmt.Errorf("update deployment device error: %w", err)
		}

		// if all devices have finished the multicast class-c session setup, publish to done chan.
		done := true
		for _, state := range d.deviceState {
			// ignore devices that have not setup the fragmentation session
			if !state.getFragmentationSessionSetup() {
				continue
			}

			if !state.getMulticastSessionSetup() {
				done = false
			}
		}
		if done {
			d.multicastSessionSetupDone <- struct{}{}
		}
	}

	return nil
}

func (d *Deployment) handleFragSessionStatusAns(ctx context.Context, devEUI lorawan.EUI64, pl *fragmentation.FragSessionStatusAnsPayload) error {
	log.WithFields(log.Fields{
		"deployment_id":            d.GetID(),
		"dev_eui":                  devEUI,
		"frag_index":               pl.ReceivedAndIndex.FragIndex,
		"nb_frag_received":         pl.ReceivedAndIndex.NbFragReceived,
		"missing_frag":             pl.MissingFrag,
		"not_enough_matrix_memory": pl.Status.NotEnoughMatrixMemory,
	}).Info("fuota: FragSessionStatusAns received")

	dl := storage.DeploymentLog{
		DeploymentID: d.GetID(),
		DevEUI:       devEUI,
		FPort:        fragmentation.DefaultFPort,
		Command:      "FragSessionStatusAns",
		Fields: hstore.Hstore{
			Map: map[string]sql.NullString{
				"frag_index":               sql.NullString{Valid: true, String: fmt.Sprintf("%d", pl.ReceivedAndIndex.FragIndex)},
				"nb_frag_received":         sql.NullString{Valid: true, String: fmt.Sprintf("%d", pl.ReceivedAndIndex.NbFragReceived)},
				"missing_frag":             sql.NullString{Valid: true, String: fmt.Sprintf("%d", pl.MissingFrag)},
				"not_enough_matrix_memory": sql.NullString{Valid: true, String: fmt.Sprintf("%t", pl.Status.NotEnoughMatrixMemory)},
			},
		},
	}
	if err := storage.CreateDeploymentLog(ctx, storage.DB(), &dl); err != nil {
		log.WithError(err).Error("fuota: create deployment log error")
	}

	if pl.ReceivedAndIndex.FragIndex == d.opts.FragmentationSessionIndex && pl.MissingFrag == 0 && !pl.Status.NotEnoughMatrixMemory {
		// update the device state
		if state, ok := d.deviceState[devEUI]; ok {
			state.setFragmentationSessionStatus(true)
		}

		dd, err := storage.GetDeploymentDevice(ctx, storage.DB(), d.GetID(), devEUI)
		if err != nil {
			return fmt.Errorf("get deployment device error: %w", err)
		}
		now := time.Now()
		dd.FragStatusCompletedAt = &now
		if err := storage.UpdateDeploymentDevice(ctx, storage.DB(), &dd); err != nil {
			return fmt.Errorf("update deployment device error: %w", err)
		}

		// if all devices have finished the frag session status, publish to done chan.
		done := true
		for _, state := range d.deviceState {
			// ignore devices that do not have the multicast-session setup
			if !state.getMulticastSessionSetup() {
				continue
			}

			if !state.getFragmentationSessionStatus() {
				done = false
			}
		}
		if done {
			d.fragmentationSessionStatusDone <- struct{}{}
		}
	}

	return nil
}

// create multicast group step
func (d *Deployment) stepCreateMulticastGroup(ctx context.Context) error {
	log.WithField("deployment_id", d.GetID()).Debug("fuota: stepCreateMulticastGroup funtion called")

	// generate randomd devaddr
	if _, err := rand.Read(d.mcAddr[:]); err != nil {
		return fmt.Errorf("read random bytes error: %w", err)
	}

	// generate random McKey
	if _, err := rand.Read(d.mcKey[:]); err != nil {
		return fmt.Errorf("read random bytes error: %w", err)
	}

	// get McAppSKey
	mcAppSKey, err := multicastsetup.GetMcAppSKey(d.mcKey, d.mcAddr)
	if err != nil {
		return fmt.Errorf("get McAppSKey error: %w", err)
	}

	// get McNetSKey
	mcNetSKey, err := multicastsetup.GetMcNetSKey(d.mcKey, d.mcAddr)
	if err != nil {
		return fmt.Errorf("get McNetSKey error: %s", err)
	}

	mg := api.MulticastGroup{
		Name:                 fmt.Sprintf("fuota-%s", d.GetID()),
		McAddr:               d.mcAddr.String(),
		McNwkSKey:            mcNetSKey.String(),
		McAppSKey:            mcAppSKey.String(),
		GroupType:            d.opts.MulticastGroupType,
		Dr:                   uint32(d.opts.MulticastDR),
		Frequency:            d.opts.MulticastFrequency,
		ClassBPingSlotPeriod: uint32(1 << int(5+d.opts.MulticastPingSlotPeriodicity)), // note: period = 2 ^ (5 + periodicity)
		ApplicationId:        d.opts.ApplicationID,
		Region:               d.opts.MulticastRegion,
	}

	resp, err := as.MulticastGroupClient().Create(ctx, &api.CreateMulticastGroupRequest{
		MulticastGroup: &mg,
	})
	if err != nil {
		return fmt.Errorf("create multicast-group error: %w", err)
	}

	d.multicastGroupID = resp.Id

	log.WithFields(log.Fields{
		"deployment_id":      d.GetID(),
		"multicast_group_id": resp.Id,
	}).Info("fuota: multicast-group created")

	return nil
}

// Wait until multicast-session timeout.
// This is needed in case the fragmentation-session status request step is skipped.
// We don't want to cleanup the multicast-group before the multicast-session has
// expired.
func (d *Deployment) stepWaitUntilTimeout(ctx context.Context) error {
	timeDiff := d.sessionEndTime.Sub(time.Now())
	if timeDiff > 0 {
		log.WithFields(log.Fields{
			"deployment_id": d.GetID(),
			"sleep_time":    timeDiff,
		}).Info("fuota: waiting for multicast-session to end for devices")
		time.Sleep(timeDiff)
	}

	return nil
}

// delete multicast-group.
func (d *Deployment) stepDeleteMulticastGroup(ctx context.Context) error {
	log.WithField("deployment_id", d.GetID()).Info("fuota: deleting multicast-group")

	_, err := as.MulticastGroupClient().Delete(ctx, &api.DeleteMulticastGroupRequest{
		Id: d.multicastGroupID,
	})
	if err != nil {
		return fmt.Errorf("delete multicast-group error: %w", err)
	}

	log.WithFields(log.Fields{
		"deployment_id":      d.GetID(),
		"multicast_group_id": d.multicastGroupID,
	}).Info("fuota: multicast-group deleted")

	return nil
}

// add devices to multicast-group
func (d *Deployment) stepAddDevicesToMulticastGroup(ctx context.Context) error {
	log.WithField("deployment_id", d.GetID()).Info("fuota: add devices to multicast-group")

	for devEUI := range d.opts.Devices {
		log.WithFields(log.Fields{
			"deployment_id":      d.GetID(),
			"dev_eui":            devEUI,
			"multicast_group_id": d.multicastGroupID,
		}).Info("fuota: add device to multicast-group")

		_, err := as.MulticastGroupClient().AddDevice(ctx, &api.AddDeviceToMulticastGroupRequest{
			MulticastGroupId: d.multicastGroupID,
			DevEui:           devEUI.String(),
		})
		if err != nil {
			return fmt.Errorf("add device to multicast-group error: %w", err)
		}
	}

	return nil
}

func (d *Deployment) stepMulticastSetup(ctx context.Context) error {
	log.WithField("deployment_id", d.GetID()).Info("fuota: starting multicast-setup for devices")

	attempt := 0

devLoop:
	for {
		attempt += 1
		if attempt > d.opts.UnicastAttemptCount {
			log.WithField("deployment_id", d.GetID()).Warning("fuota: multicast-setup reached max. number of attepts, some devices did not complete")
			break
		}

		for devEUI := range d.opts.Devices {
			if d.deviceState[devEUI].getMulticastSetup() {
				continue
			}

			log.WithFields(log.Fields{
				"deployment_id": d.GetID(),
				"dev_eui":       devEUI,
				"attempt":       attempt,
			}).Info("fuota: initiate multicast-setup for device")

			// get the encrypted McKey.
			var mcKeyEncrypted lorawan.AES128Key
			mcKEKey, err := multicastsetup.GetMcKEKey(d.opts.Devices[devEUI].McRootKey)
			if err != nil {
				return fmt.Errorf("GetMcKEKey error: %w", err)
			}
			block, err := aes.NewCipher(mcKEKey[:])
			if err != nil {
				return fmt.Errorf("new cipher error: %w", err)
			}
			block.Decrypt(mcKeyEncrypted[:], d.mcKey[:])

			cmd := multicastsetup.Command{
				CID: multicastsetup.McGroupSetupReq,
				Payload: &multicastsetup.McGroupSetupReqPayload{
					McGroupIDHeader: multicastsetup.McGroupSetupReqPayloadMcGroupIDHeader{
						McGroupID: d.opts.MulticastGroupID,
					},
					McAddr:         d.mcAddr,
					McKeyEncrypted: mcKeyEncrypted,
					MinMcFCnt:      0,
					MaxMcFCnt:      (1 << 32) - 1,
				},
			}

			b, err := cmd.MarshalBinary()
			if err != nil {
				return fmt.Errorf("marshal binary error: %w", err)
			}

			_, err = as.DeviceClient().Enqueue(ctx, &api.EnqueueDeviceQueueItemRequest{
				QueueItem: &api.DeviceQueueItem{
					DevEui: devEUI.String(),
					FPort:  uint32(multicastsetup.DefaultFPort),
					Data:   b,
				},
			})
			if err != nil {
				log.WithError(err).WithFields(log.Fields{
					"deployment_id": d.GetID(),
					"dev_eui":       devEUI,
				}).Error("fuota: enqueue payload error")
				continue
			}

			dl := storage.DeploymentLog{
				DeploymentID: d.GetID(),
				DevEUI:       devEUI,
				FPort:        uint8(multicastsetup.DefaultFPort),
				Command:      "McGroupSetupReq",
				Fields: hstore.Hstore{
					Map: map[string]sql.NullString{
						"mc_group_id":      sql.NullString{Valid: true, String: fmt.Sprintf("%d", d.opts.MulticastGroupID)},
						"mc_addr":          sql.NullString{Valid: true, String: d.mcAddr.String()},
						"mc_key_encrypted": sql.NullString{Valid: true, String: mcKeyEncrypted.String()},
						"min_mc_fcnt":      sql.NullString{Valid: true, String: fmt.Sprintf("%d", 0)},
						"max_mc_fcnt":      sql.NullString{Valid: true, String: fmt.Sprintf("%d", uint32((1<<32)-1))},
					},
				},
			}
			if err := storage.CreateDeploymentLog(ctx, storage.DB(), &dl); err != nil {
				log.WithError(err).Error("fuota: create deployment log error")
			}
		}

		select {
		// sleep until next retry
		case <-time.After(d.opts.UnicastTimeout):
			continue devLoop
		// terminate when all devices have been setup
		case <-d.multicastSetupDone:
			log.WithField("deployment_id", d.GetID()).Info("fuota: multicast-setup completed successful for all devices")
			break devLoop
		}
	}

	sd, err := storage.GetDeployment(ctx, storage.DB(), d.GetID())
	if err != nil {
		return fmt.Errorf("get deployment error: %w", err)
	}
	now := time.Now()
	sd.MCGroupSetupCompletedAt = &now
	if err := storage.UpdateDeployment(ctx, storage.DB(), &sd); err != nil {
		return fmt.Errorf("update deployment error: %w", err)
	}

	return nil
}

func (d *Deployment) stepFragmentationSessionSetup(ctx context.Context) error {
	log.WithField("deployment_id", d.GetID()).Info("fuota: starting fragmentation-session setup for devices")

	attempt := 0
	padding := (d.opts.FragSize - (len(d.opts.Payload) % d.opts.FragSize)) % d.opts.FragSize
	nbFrag := (len(d.opts.Payload) + padding) / d.opts.FragSize

devLoop:
	for {
		attempt += 1
		if attempt > d.opts.UnicastAttemptCount {
			log.WithField("deployment_id", d.GetID()).Warning("fuota: fragmentation-session setup reached max. number of attempts, some devices did not complete")
			break
		}

		for devEUI := range d.opts.Devices {
			// ignore devices that have not setup multicast
			if !d.deviceState[devEUI].getMulticastSetup() {
				continue
			}

			if d.deviceState[devEUI].getFragmentationSessionSetup() {
				continue
			}

			log.WithFields(log.Fields{
				"deployment_id": d.GetID(),
				"dev_eui":       devEUI,
				"attempt":       attempt,
			}).Info("fuota: initiate fragmentation-session setup for device")

			cmd := fragmentation.Command{
				CID: fragmentation.FragSessionSetupReq,
				Payload: &fragmentation.FragSessionSetupReqPayload{
					FragSession: fragmentation.FragSessionSetupReqPayloadFragSession{
						FragIndex:      d.opts.FragmentationSessionIndex,
						McGroupBitMask: [4]bool{d.opts.MulticastGroupID == 0, d.opts.MulticastGroupID == 1, d.opts.MulticastGroupID == 2, d.opts.MulticastGroupID == 3},
					},
					NbFrag:   uint16(nbFrag),
					FragSize: uint8(d.opts.FragSize),
					Control: fragmentation.FragSessionSetupReqPayloadControl{
						FragmentationMatrix: d.opts.FragmentationMatrix,
						BlockAckDelay:       d.opts.BlockAckDelay,
					},
					Padding:    uint8(padding),
					Descriptor: d.opts.Descriptor,
				},
			}

			b, err := cmd.MarshalBinary()
			if err != nil {
				return fmt.Errorf("marshal binary error: %w", err)
			}

			_, err = as.DeviceClient().Enqueue(ctx, &api.EnqueueDeviceQueueItemRequest{
				QueueItem: &api.DeviceQueueItem{
					DevEui: devEUI.String(),
					FPort:  uint32(fragmentation.DefaultFPort),
					Data:   b,
				},
			})
			if err != nil {
				log.WithError(err).WithFields(log.Fields{
					"deployment_id": d.GetID(),
					"dev_eui":       devEUI,
				}).Error("fuota: enqueue payload error")
				continue
			}

			dl := storage.DeploymentLog{
				DeploymentID: d.GetID(),
				DevEUI:       devEUI,
				FPort:        uint8(fragmentation.DefaultFPort),
				Command:      "FragSessionSetupReq",
				Fields: hstore.Hstore{
					Map: map[string]sql.NullString{
						"frag_index":           sql.NullString{Valid: true, String: fmt.Sprintf("%d", d.opts.FragmentationSessionIndex)},
						"McGroupBitMask":       sql.NullString{Valid: true, String: fmt.Sprintf("%d", uint32(1<<d.opts.MulticastGroupID))},
						"nb_frag":              sql.NullString{Valid: true, String: fmt.Sprintf("%d", nbFrag)},
						"frag_size":            sql.NullString{Valid: true, String: fmt.Sprintf("%d", d.opts.FragSize)},
						"fragmentation_matrix": sql.NullString{Valid: true, String: fmt.Sprintf("%d", d.opts.FragmentationMatrix)},
						"block_ack_delay":      sql.NullString{Valid: true, String: fmt.Sprintf("%d", d.opts.BlockAckDelay)},
						"padding":              sql.NullString{Valid: true, String: fmt.Sprintf("%d", padding)},
						"descriptor":           sql.NullString{Valid: true, String: hex.EncodeToString(d.opts.Descriptor[:])},
					},
				},
			}
			if err := storage.CreateDeploymentLog(ctx, storage.DB(), &dl); err != nil {
				log.WithError(err).Error("fuota: create deployment log error")
			}
		}

		select {
		// sleep until next retry
		case <-time.After(d.opts.UnicastTimeout):
			continue devLoop
			// terminate when all devices have been setup
		case <-d.fragmentationSessionSetupDone:
			log.WithField("deployment_id", d.GetID()).Info("fuota: fragmentation-session setup completed successful for all devices")
			break devLoop
		}
	}

	sd, err := storage.GetDeployment(ctx, storage.DB(), d.GetID())
	if err != nil {
		return fmt.Errorf("get deployment error: %w", err)
	}
	now := time.Now()
	sd.FragSessionSetupCompletedAt = &now
	if err := storage.UpdateDeployment(ctx, storage.DB(), &sd); err != nil {
		return fmt.Errorf("update deployment error: %w", err)
	}

	return nil
}

func (d *Deployment) stepMulticastClassBSessionSetup(ctx context.Context) error {
	if d.opts.MulticastGroupType != api.MulticastGroupType_CLASS_B {
		return nil
	}

	log.WithField("deployment_id", d.GetID()).Info("fuota: starting multicast class-b session setup for devices")

	attempt := 0

devLoop:
	for {
		attempt += 1
		if attempt > d.opts.UnicastAttemptCount {
			log.WithField("deployment_id", d.GetID()).Warning("fuota: multicast class-b session setup reached max. number of attempts, some devices did not complete")
			break
		}

		d.sessionStartTime = time.Now().Add(d.opts.UnicastTimeout)
		d.sessionEndTime = d.sessionStartTime.Add(time.Duration(1<<d.opts.MulticastTimeout) * time.Second)

		for devEUI := range d.opts.Devices {
			// ignore devices that have not setup the fragmentation session
			if !d.deviceState[devEUI].getFragmentationSessionSetup() {
				continue
			}

			if d.deviceState[devEUI].getMulticastSessionSetup() {
				continue
			}

			log.WithFields(log.Fields{
				"deployment_id": d.GetID(),
				"dev_eui":       devEUI,
				"attempt":       attempt,
			}).Info("fuota: initiate multicast class-b session setup for device")

			sessionTime := uint32((gps.Time(d.sessionStartTime).TimeSinceGPSEpoch() / time.Second) % (1 << 32))

			cmd := multicastsetup.Command{
				CID: multicastsetup.McClassBSessionReq,
				Payload: &multicastsetup.McClassBSessionReqPayload{
					McGroupIDHeader: multicastsetup.McClassBSessionReqPayloadMcGroupIDHeader{
						McGroupID: d.opts.MulticastGroupID,
					},
					SessionTime: sessionTime,
					TimeOutPeriodicity: multicastsetup.McClassBSessionReqPayloadTimeOutPeriodicity{
						Periodicity: d.opts.MulticastPingSlotPeriodicity,
						TimeOut:     d.opts.MulticastTimeout,
					},
					DLFrequency: d.opts.MulticastFrequency,
					DR:          d.opts.MulticastDR,
				},
			}

			b, err := cmd.MarshalBinary()
			if err != nil {
				return fmt.Errorf("marshal binary error: %w", err)
			}

			_, err = as.DeviceClient().Enqueue(ctx, &api.EnqueueDeviceQueueItemRequest{
				QueueItem: &api.DeviceQueueItem{
					DevEui: devEUI.String(),
					FPort:  uint32(multicastsetup.DefaultFPort),
					Data:   b,
				},
			})
			if err != nil {
				log.WithError(err).WithFields(log.Fields{
					"deployment_id": d.GetID(),
					"dev_eui":       devEUI,
				}).Error("fuota: enqueue payload error")
				continue
			}

			dl := storage.DeploymentLog{
				DeploymentID: d.GetID(),
				DevEUI:       devEUI,
				FPort:        uint8(multicastsetup.DefaultFPort),
				Command:      "McClassBSessionReq",
				Fields: hstore.Hstore{
					Map: map[string]sql.NullString{
						"mc_group_id":         sql.NullString{Valid: true, String: fmt.Sprintf("%d", d.opts.MulticastGroupID)},
						"session_time":        sql.NullString{Valid: true, String: fmt.Sprintf("%d", sessionTime)},
						"session_periodicity": sql.NullString{Valid: true, String: fmt.Sprintf("%d", d.opts.MulticastPingSlotPeriodicity)},
						"session_time_out":    sql.NullString{Valid: true, String: fmt.Sprintf("%d", d.opts.MulticastTimeout)},
						"dl_frequency":        sql.NullString{Valid: true, String: fmt.Sprintf("%d", d.opts.MulticastFrequency)},
						"dr":                  sql.NullString{Valid: true, String: fmt.Sprintf("%d", d.opts.MulticastDR)},
					},
				},
			}
			if err := storage.CreateDeploymentLog(ctx, storage.DB(), &dl); err != nil {
				log.WithError(err).Error("fuota: create deployment log error")
			}
		}

		select {
		// sleep until next retry
		case <-time.After(d.opts.UnicastTimeout):
			continue devLoop
		case <-d.multicastSessionSetupDone:
			log.WithField("deployment_id", d.GetID()).Info("fuota: multicast class-b session setup completed successful for all devices")
			break devLoop
		}
	}

	sd, err := storage.GetDeployment(ctx, storage.DB(), d.GetID())
	if err != nil {
		return fmt.Errorf("get deployment error: %w", err)
	}
	now := time.Now()
	sd.MCSessionCompletedAt = &now
	if err := storage.UpdateDeployment(ctx, storage.DB(), &sd); err != nil {
		return fmt.Errorf("update deployment error: %w", err)
	}

	return nil
}

func (d *Deployment) stepMulticastClassCSessionSetup(ctx context.Context) error {
	if d.opts.MulticastGroupType != api.MulticastGroupType_CLASS_C {
		return nil
	}

	log.WithField("deployment_id", d.GetID()).Info("fuota: starting multicast class-c session setup for devices")

	attempt := 0

devLoop:
	for {
		attempt += 1
		if attempt > d.opts.UnicastAttemptCount {
			log.WithField("deployment_id", d.GetID()).Warning("fuota: multicast class-c session setup reached max. number of attempts, some devices did not complete")
			break
		}

		d.sessionStartTime = time.Now().Add(d.opts.UnicastTimeout)
		d.sessionEndTime = d.sessionStartTime.Add(time.Duration(1<<d.opts.MulticastTimeout) * time.Second)

		for devEUI := range d.opts.Devices {
			// ignore devices that have not setup the fragmentation session
			if !d.deviceState[devEUI].getFragmentationSessionSetup() {
				continue
			}

			if d.deviceState[devEUI].getMulticastSessionSetup() {
				continue
			}

			log.WithFields(log.Fields{
				"deployment_id": d.GetID(),
				"dev_eui":       devEUI,
				"attempt":       attempt,
			}).Info("fuota: initiate multicast class-c session setup for device")

			sessionTime := uint32((gps.Time(d.sessionStartTime).TimeSinceGPSEpoch() / time.Second) % (1 << 32))

			cmd := multicastsetup.Command{
				CID: multicastsetup.McClassCSessionReq,
				Payload: &multicastsetup.McClassCSessionReqPayload{
					McGroupIDHeader: multicastsetup.McClassCSessionReqPayloadMcGroupIDHeader{
						McGroupID: d.opts.MulticastGroupID,
					},
					SessionTime: sessionTime,
					SessionTimeOut: multicastsetup.McClassCSessionReqPayloadSessionTimeOut{
						TimeOut: d.opts.MulticastTimeout,
					},
					DLFrequency: d.opts.MulticastFrequency,
					DR:          d.opts.MulticastDR,
				},
			}

			b, err := cmd.MarshalBinary()
			if err != nil {
				return fmt.Errorf("marshal binary error: %w", err)
			}

			_, err = as.DeviceClient().Enqueue(ctx, &api.EnqueueDeviceQueueItemRequest{
				QueueItem: &api.DeviceQueueItem{
					DevEui: devEUI.String(),
					FPort:  uint32(multicastsetup.DefaultFPort),
					Data:   b,
				},
			})
			if err != nil {
				log.WithError(err).WithFields(log.Fields{
					"deployment_id": d.GetID(),
					"dev_eui":       devEUI,
				}).Error("fuota: enqueue payload error")
				continue
			}

			dl := storage.DeploymentLog{
				DeploymentID: d.GetID(),
				DevEUI:       devEUI,
				FPort:        uint8(multicastsetup.DefaultFPort),
				Command:      "McClassCSessionReq",
				Fields: hstore.Hstore{
					Map: map[string]sql.NullString{
						"mc_group_id":      sql.NullString{Valid: true, String: fmt.Sprintf("%d", d.opts.MulticastGroupID)},
						"session_time":     sql.NullString{Valid: true, String: fmt.Sprintf("%d", sessionTime)},
						"session_time_out": sql.NullString{Valid: true, String: fmt.Sprintf("%d", d.opts.MulticastTimeout)},
						"dl_frequency":     sql.NullString{Valid: true, String: fmt.Sprintf("%d", d.opts.MulticastFrequency)},
						"dr":               sql.NullString{Valid: true, String: fmt.Sprintf("%d", d.opts.MulticastDR)},
					},
				},
			}
			if err := storage.CreateDeploymentLog(ctx, storage.DB(), &dl); err != nil {
				log.WithError(err).Error("fuota: create deployment log error")
			}
		}

		select {
		// sleep until next retry
		case <-time.After(d.opts.UnicastTimeout):
			continue devLoop
		case <-d.multicastSessionSetupDone:
			log.WithField("deployment_id", d.GetID()).Info("fuota: multicast class-c session setup completed successful for all devices")
			break devLoop
		}
	}

	sd, err := storage.GetDeployment(ctx, storage.DB(), d.GetID())
	if err != nil {
		return fmt.Errorf("get deployment error: %w", err)
	}
	now := time.Now()
	sd.MCSessionCompletedAt = &now
	if err := storage.UpdateDeployment(ctx, storage.DB(), &sd); err != nil {
		return fmt.Errorf("update deployment error: %w", err)
	}

	return nil
}

func (d *Deployment) stepEnqueue(ctx context.Context) error {
	log.WithField("deployment_id", d.GetID()).Info("fuota: starting multicast enqueue")

	timeDiff := d.sessionStartTime.Sub(time.Now())
	if timeDiff > 0 {
		log.WithFields(log.Fields{
			"deployment_id": d.GetID(),
			"sleep_time":    timeDiff,
		}).Info("fuota: waiting with enqueue until multicast-session starts")
		time.Sleep(timeDiff)
	}

	// fragment the payload
	padding := (d.opts.FragSize - (len(d.opts.Payload) % d.opts.FragSize)) % d.opts.FragSize
	fragments, err := fragmentation.Encode(append(d.opts.Payload, make([]byte, padding)...), d.opts.FragSize, d.opts.Redundancy)
	if err != nil {
		return fmt.Errorf("fragment payload error: %w", err)
	}

	// wrap the payloads into data-fragment payloads
	var payloads [][]byte
	for i := range fragments {
		cmd := fragmentation.Command{
			CID: fragmentation.DataFragment,
			Payload: &fragmentation.DataFragmentPayload{
				IndexAndN: fragmentation.DataFragmentPayloadIndexAndN{
					FragIndex: uint8(d.opts.FragmentationSessionIndex),
					N:         uint16(i + 1),
				},
				Payload: fragments[i],
			},
		}

		b, err := cmd.MarshalBinary()
		if err != nil {
			return fmt.Errorf("marshal binary error: %w", err)
		}

		payloads = append(payloads, b)
	}

	// enqueue the payloads
	for i, b := range payloads {
		_, err = as.MulticastGroupClient().Enqueue(ctx, &api.EnqueueMulticastGroupQueueItemRequest{
			QueueItem: &api.MulticastGroupQueueItem{
				MulticastGroupId: d.multicastGroupID,
				FCnt:             uint32(i),
				FPort:            uint32(fragmentation.DefaultFPort),
				Data:             b,
			},
		})
	}

	sd, err := storage.GetDeployment(ctx, storage.DB(), d.GetID())
	if err != nil {
		return fmt.Errorf("get deployment error: %w", err)
	}
	now := time.Now()
	sd.EnqueueCompletedAt = &now
	if err := storage.UpdateDeployment(ctx, storage.DB(), &sd); err != nil {
		return fmt.Errorf("update deployment error: %w", err)
	}

	return nil
}

func (d *Deployment) stepFragSessionStatus(ctx context.Context) error {
	if d.opts.RequestFragmentationSessionStatus == RequestFragmentationSessionStatusNoRequest {
		log.WithField("deployment_id", d.GetID()).Info("fuota: skipping fragmentation-session status request as requested")
		return nil
	}

	if d.opts.RequestFragmentationSessionStatus == RequestFragmentationSessionStatusAfterSessionTimeout {
		timeDiff := d.sessionEndTime.Sub(time.Now())
		if timeDiff > 0 {
			log.WithFields(log.Fields{
				"deployment_id": d.GetID(),
				"sleep_time":    timeDiff,
			}).Info("fuota: waiting for multicast-session to end for devices before sending fragmentation-session status request")
			time.Sleep(timeDiff)
		}
	}

	log.WithField("deployment_id", d.GetID()).Info("fuota: starting fragmentation-session status request for devices")

	attempt := 0

devLoop:
	for {
		attempt += 1
		if attempt > d.opts.UnicastAttemptCount {
			log.WithField("deployment_id", d.GetID()).Warning("fuota: fragmentation-session status request reached max. number of attempts, some devices did not complete")
			break
		}

		for devEUI := range d.opts.Devices {
			// ignore devices that do not have the multicast-session setup
			if !d.deviceState[devEUI].getMulticastSessionSetup() {
				continue
			}

			if d.deviceState[devEUI].getFragmentationSessionStatus() {
				continue
			}

			log.WithFields(log.Fields{
				"deployment_id": d.GetID(),
				"dev_eui":       devEUI,
				"attempt":       attempt,
			}).Info("fuota: request fragmentation-session status for device")

			cmd := fragmentation.Command{
				CID: fragmentation.FragSessionStatusReq,
				Payload: &fragmentation.FragSessionStatusReqPayload{
					FragStatusReqParam: fragmentation.FragSessionStatusReqPayloadFragStatusReqParam{
						FragIndex:    d.opts.FragmentationSessionIndex,
						Participants: true,
					},
				},
			}

			b, err := cmd.MarshalBinary()
			if err != nil {
				return fmt.Errorf("marshal binary error: %w", err)
			}

			_, err = as.DeviceClient().Enqueue(ctx, &api.EnqueueDeviceQueueItemRequest{
				QueueItem: &api.DeviceQueueItem{
					DevEui: devEUI.String(),
					FPort:  uint32(fragmentation.DefaultFPort),
					Data:   b,
				},
			})
			if err != nil {
				log.WithError(err).WithFields(log.Fields{
					"deployment_id": d.GetID(),
					"dev_eui":       devEUI,
				}).Error("fuota: enqueue payload error")
			}

			dl := storage.DeploymentLog{
				DeploymentID: d.GetID(),
				DevEUI:       devEUI,
				FPort:        uint8(fragmentation.DefaultFPort),
				Command:      "FragSessionStatusReq",
				Fields: hstore.Hstore{
					Map: map[string]sql.NullString{
						"frag_index":   sql.NullString{Valid: true, String: fmt.Sprintf("%d", d.opts.FragmentationSessionIndex)},
						"participants": sql.NullString{Valid: true, String: "true"},
					},
				},
			}
			if err := storage.CreateDeploymentLog(ctx, storage.DB(), &dl); err != nil {
				log.WithError(err).Error("fuota: create deployment log error")
			}
		}

		// wait until multicast-session has ended for all devices
		if d.opts.RequestFragmentationSessionStatus != RequestFragmentationSessionStatusAfterSessionTimeout {
			timeDiff := d.sessionEndTime.Sub(time.Now())
			if timeDiff > 0 {
				log.WithFields(log.Fields{
					"deployment_id": d.GetID(),
					"sleep_time":    timeDiff,
				}).Info("fuota: waiting for multicast-session to end for devices")
				time.Sleep(timeDiff)
			}
		}

		select {
		// sleep until next retry
		case <-time.After(d.opts.UnicastTimeout):
			continue devLoop
		case <-d.fragmentationSessionStatusDone:
			log.WithField("deployment_id", d.GetID()).Info("fuota: fragmentation-session status request completed successful for all devices")
			break devLoop
		}
	}

	sd, err := storage.GetDeployment(ctx, storage.DB(), d.GetID())
	if err != nil {
		return fmt.Errorf("get deployment error: %w", err)
	}
	now := time.Now()
	sd.FragStatusCompletedAt = &now
	if err := storage.UpdateDeployment(ctx, storage.DB(), &sd); err != nil {
		return fmt.Errorf("update deployment error: %w", err)
	}

	return nil
}

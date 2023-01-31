package eventhandler

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"

	"github.com/gofrs/uuid"
	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	log "github.com/sirupsen/logrus"

	"github.com/brocaar/lorawan/applayer/clocksync"
	"github.com/chirpstack/chirpstack-fuota-server/v4/internal/config"
	"github.com/chirpstack/chirpstack/api/go/v4/integration"
)

var handler *Handler

// Get returns the Handler.
func Get() *Handler {
	if handler == nil {
		panic("Handler is not initialized")
	}

	return handler
}

// Setup configures the Handler.
func Setup(c *config.Config) error {
	log.Info("eventhandler: setup application-server event-handler")

	opts := HandlerOptions{}
	switch c.ChirpStack.EventHandler.Marshaler {
	case "json":
		opts.JSON = true
	case "protobuf":
		opts.JSON = false
	default:
		return fmt.Errorf("invalid marshaler option: %s", c.ChirpStack.EventHandler.Marshaler)
	}

	h, err := NewHandler(opts)
	if err != nil {
		return fmt.Errorf("new handler error: %w", err)
	}

	handler = h
	server := http.Server{
		Handler: handler,
		Addr:    c.ChirpStack.EventHandler.HTTP.Bind,
	}

	go func() {
		log.WithFields(log.Fields{
			"bind":      c.ChirpStack.EventHandler.HTTP.Bind,
			"marshaler": c.ChirpStack.EventHandler.Marshaler,
		}).Info("integration/eventhandler: starting event-handler server")
		err := server.ListenAndServe()
		log.WithError(err).Error("eventhandler: start event-handler server error")
	}()

	return nil
}

type HandlerOptions struct {
	// JSON indicates if the events are in JSON format. When left to false
	// Protobuf is expected.
	JSON bool
}

// Handler provides event handling.
type Handler struct {
	sync.RWMutex

	opts          HandlerOptions
	eventHandlers map[uuid.UUID]func(context.Context, integration.UplinkEvent) error
}

// NewHandler creates a new Handler.
func NewHandler(opts HandlerOptions) (*Handler, error) {
	return &Handler{
		opts:          opts,
		eventHandlers: make(map[uuid.UUID]func(context.Context, integration.UplinkEvent) error),
	}, nil
}

// RegisterUplinkEventFunc registers the given function to handle uplink events.
// Multiple functions can be registered simultaneously.
func (h *Handler) RegisterUplinkEventFunc(id uuid.UUID, f func(context.Context, integration.UplinkEvent) error) {
	h.Lock()
	defer h.Unlock()

	h.eventHandlers[id] = f
}

// UnregisterUplinkEventFunc removes the function to handle uplink events.
func (h *Handler) UnregisterUplinkEventFunc(id uuid.UUID) {
	h.Lock()
	defer h.Unlock()

	delete(h.eventHandlers, id)
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	event := r.URL.Query().Get("event")

	// ignore non-uplink events
	if event != "up" {
		return
	}

	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.WithError(err).Error("eventhandler: read request body error")
		return
	}

	var uplinkEvent integration.UplinkEvent
	if err := h.unmarshal(b, &uplinkEvent); err != nil {
		log.WithError(err).Error("eventhandler: unmarshal UplinkEvent error")
		return
	}

	log.WithFields(log.Fields{
		"event":   event,
		"dev_eui": uplinkEvent.GetDeviceInfo().GetDevEui(),
		"f_cnt":   uplinkEvent.FCnt,
		"f_port":  uplinkEvent.FPort,
		"data":    hex.EncodeToString(uplinkEvent.Data),
	}).Debug("eventhandler: event received from application-server")

	h.RLock()
	defer h.RUnlock()

	if uint8(uplinkEvent.FPort) == clocksync.DefaultFPort {
		// Handle the clocksync in any case as it is requested by the device. Depending
		// the device implementation, the request might be received even before the
		// FUOTA deployment is created.
		go func(pl integration.UplinkEvent) {
			if err := handleClockSyncCommand(context.Background(), pl); err != nil {
				log.WithError(err).Error("handle clocksync error")
			}
		}(uplinkEvent)

	} else {
		for id, f := range h.eventHandlers {
			go func(pl integration.UplinkEvent) {
				if err := f(context.Background(), pl); err != nil {
					log.WithError(err).WithField("id", id).Error("integration/eventhandler: uplink event handler error")
				}
			}(uplinkEvent)
		}
	}

}

func (h *Handler) unmarshal(b []byte, v proto.Message) error {
	if h.opts.JSON {
		unmarshaler := &jsonpb.Unmarshaler{
			AllowUnknownFields: true, // we don't want to fail on unknown fields
		}
		return unmarshaler.Unmarshal(bytes.NewReader(b), v)
	}
	return proto.Unmarshal(b, v)
}

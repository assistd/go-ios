package ioskit

import (
	"context"
	"fmt"
	"sync"

	"github.com/danielpaulus/go-ios/ios"
	log "github.com/sirupsen/logrus"
)

type DeviceEntry ios.DeviceEntry

var emptyDevice DeviceEntry

type (
	propertyKey struct {
		d DeviceEntry
		k interface{}
	}
)

type Ctx struct {
	ctx    context.Context
	cancel context.CancelFunc
}

// Registry is holds a list of registered devices. It provides methods for
// listening for devices that are added to and removed from the device.
type Registry struct {
	sync.Mutex
	devices    []DeviceEntry
	ctxMap     map[int]Ctx
	properties map[propertyKey]interface{}
	listeners  map[DeviceListener]struct{}
}

// NewRegistry returns a newly constructed Registry.
func NewRegistry() *Registry {
	return &Registry{
		listeners:  make(map[DeviceListener]struct{}),
		properties: make(map[propertyKey]interface{}),
	}
}

// DeviceListener is the interface implemented by types that respond to devices
// being added to and removed from the registry.
type DeviceListener interface {
	OnDeviceAdded(context.Context, DeviceEntry)
	OnDeviceRemoved(context.Context, DeviceEntry)
}

// NewDeviceListener returns a DeviceListener that delegates calls on to
// onDeviceAdded and onDeviceRemoved.
func NewDeviceListener(onDeviceAdded, onDeviceRemoved func(context.Context, DeviceEntry)) DeviceListener {
	return &funcDeviceListener{onDeviceAdded, onDeviceRemoved}
}

// funcDeviceListener is an implementatation of DeviceListener that delegates
// calls on to the field functions.
type funcDeviceListener struct {
	onAdded   func(context.Context, DeviceEntry)
	onRemoved func(context.Context, DeviceEntry)
}

func (l funcDeviceListener) OnDeviceAdded(ctx context.Context, d DeviceEntry) {
	if f := l.onAdded; f != nil {
		f(ctx, d)
	}
}

func (l funcDeviceListener) OnDeviceRemoved(ctx context.Context, d DeviceEntry) {
	if f := l.onRemoved; f != nil {
		f(ctx, d)
	}
}

// Listen registers l to be called whenever a device is added to or removed from
// the registry. l will be unregistered when the returned function is called.
func (r *Registry) Listen(l DeviceListener) (unregister func()) {
	r.Lock()
	r.listeners[l] = struct{}{}
	r.Unlock()
	return func() {
		r.Lock()
		delete(r.listeners, l)
		r.Unlock()
	}
}

// Device looks up the device with the specified identifier.
// If no device with the specified identifier was registered with the Registry
// then nil is returner.
func (r *Registry) Device(id int) (DeviceEntry, error) {
	r.Lock()
	defer r.Unlock()
	for _, d := range r.devices {
		if d.DeviceID == id {
			return d, nil
		}
	}
	return emptyDevice, fmt.Errorf("device not found")
}

// DeviceBySerial looks up the device with serial or udid
// if no device with specified serial with the Registry
// then nil is returner.
func (r *Registry) DeviceBySerial(serial string) (DeviceEntry, error) {
	r.Lock()
	defer r.Unlock()
	for _, d := range r.devices {
		if d.Properties.SerialNumber == serial {
			return d, nil
		}
	}
	return emptyDevice, fmt.Errorf("device not found")
}

// Devices returns the list of devices registered with the Registry.
func (r *Registry) Devices() []DeviceEntry {
	r.Lock()
	defer r.Unlock()

	out := make([]DeviceEntry, len(r.devices))
	copy(out, r.devices)
	return out
}

// DefaultDevice returns the first device registered with the Registry.
func (r *Registry) DefaultDevice() (DeviceEntry, error) {
	r.Lock()
	defer r.Unlock()
	if len(r.devices) == 0 {
		return emptyDevice, fmt.Errorf("no device")
	}
	return r.devices[0], nil
}

// AddDevice registers the device dev with the Registry.
func (r *Registry) AddDevice(ctx context.Context, d DeviceEntry) {
	r.Lock()
	defer r.Unlock()
	for _, t := range r.devices {
		if t == d {
			return // already added
		}

		if t.Properties.SerialNumber == d.Properties.SerialNumber && t.DeviceID != d.DeviceID {
			log.Panic("registry: add same device with different DevicdID %v, %v:%v",
				d.Properties.SerialNumber, d.Properties.SerialNumber, t.DeviceID)
		}
	}

	log.Infof("registry: adding new device, id:%v, udid:%v", d.DeviceID, d.Properties.SerialNumber)
	ctx, cancel := context.WithCancel(ctx)
	r.ctxMap[d.DeviceID] = Ctx{ctx, cancel}
	r.devices = append(r.devices, d)

	for l := range r.listeners {
		l.OnDeviceAdded(ctx, d)
	}
}

// RemoveDevice unregisters the device d with the Registry.
func (r *Registry) RemoveDevice(d DeviceEntry) {
	r.Lock()
	defer r.Unlock()
	for i, t := range r.devices {
		if t.DeviceID == d.DeviceID || t.Properties.SerialNumber == d.Properties.SerialNumber {
			log.Infof("registry: removing existing device %v", t.DeviceID, t.Properties.SerialNumber)
			copy(r.devices[i:], r.devices[i+1:])
			r.devices = r.devices[:len(r.devices)-1]

			// Invoke listeners
			ctx := r.ctxMap[t.DeviceID]
			for l := range r.listeners {
				l.OnDeviceRemoved(ctx.ctx, d)
			}
			ctx.cancel()

			// Delete ctx map
			delete(r.ctxMap, t.DeviceID)
			return
		}
	}
}

// RemoveAll unregisters all devices in the Registry.
func (r *Registry) RemoveAll() {
	r.Lock()
	defer r.Unlock()
	for i, t := range r.devices {
		log.Infof("registry: removing all [%d]:%v,%v", i, t.DeviceID, t.Properties.SerialNumber)
		ctx := r.ctxMap[t.DeviceID]
		for l := range r.listeners {
			l.OnDeviceRemoved(ctx.ctx, t)
		}
		ctx.cancel()
	}

	for k := range r.ctxMap {
		delete(r.ctxMap, k)
	}
	r.devices = make([]DeviceEntry, 0)
}

// DeviceProperty returns the property with the key k for the device d,
// previously set with SetDeviceProperty. If the property for the device does
// not exist then nil is returned.
func (r *Registry) DeviceProperty(ctx context.Context, d DeviceEntry, k interface{}) interface{} {
	r.Lock()
	defer r.Unlock()
	return r.properties[propertyKey{d, k}]
}

// SetDeviceProperty sets the property with the key k to the value v for the
// device d. This property can be retrieved with DeviceProperty.
// Properties will persist in the registry even when the device has not been
// added or has been removed.
func (r *Registry) SetDeviceProperty(ctx context.Context, d DeviceEntry, k, v interface{}) {
	r.Lock()
	defer r.Unlock()
	r.properties[propertyKey{d, k}] = v
}

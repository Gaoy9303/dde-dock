/**
 * Copyright (c) 2014 Deepin, Inc.
 *               2014 Xu FaSheng
 *
 * Author:      Xu FaSheng <fasheng.xu@gmail.com>
 * Maintainer:  Xu FaSheng <fasheng.xu@gmail.com>
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program; if not, see <http://www.gnu.org/licenses/>.
 **/

package network

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"pkg.linuxdeepin.com/lib/dbus"
	"sync"
)

var networkConfigFile = os.Getenv("HOME") + "/.config/deepin_network.json"

type config struct {
	configFile string
	saveLock   sync.Mutex

	WiredEnabled bool
	VpnEnabled   bool
	Devices      map[string]*deviceConfig
	// VpnConnections map // TODO

	LastWirelessEnabled bool
	LastWwanEnabled     bool
	LastWiredEnabled    bool
	LastVpnEnabled      bool
}

type deviceConfig struct {
	Enabled     bool
	LastEnabled bool

	// uuid of last activated connection, don't need to save it to
	// configuration file for that NetworkManager will decide which
	// connection to activate for each device after login, so just
	// follow NetworkManager's choice.
	lastConnectionUuid string
}

func newConfig() (c *config) {
	c = &config{}
	c.setConfigFile(networkConfigFile)
	c.WiredEnabled = true
	c.VpnEnabled = true
	c.Devices = make(map[string]*deviceConfig)
	c.LastWirelessEnabled = true
	c.LastWwanEnabled = true
	c.LastWiredEnabled = true
	c.LastVpnEnabled = true
	c.load()
	return
}

func newDeviceConfig() (d *deviceConfig) {
	d = &deviceConfig{}
	d.Enabled = true
	d.LastEnabled = d.Enabled
	return
}

func (c *config) setConfigFile(file string) {
	c.configFile = file
}

func (c *config) load() {
	if isFileExists(c.configFile) {
		fileContent, err := ioutil.ReadFile(c.configFile)
		if err != nil {
			logger.Error(err)
			return
		}
		unmarshalJSON(string(fileContent), c)
	} else {
		c.save()
	}
}
func (c *config) save() {
	c.saveLock.Lock()
	defer c.saveLock.Unlock()
	ensureDirExists(path.Dir(c.configFile))
	fileContent, _ := marshalJSON(c)
	err := ioutil.WriteFile(c.configFile, []byte(fileContent), 0644)
	if err != nil {
		logger.Error(err)
	}
}

func (c *config) setLastGlobalSwithes(enabled bool) {
	c.LastWirelessEnabled = enabled
	c.LastWwanEnabled = enabled
	c.LastWiredEnabled = enabled
	c.LastVpnEnabled = enabled
	c.save()
}
func (c *config) setLastWirelessEnabled(enabled bool) {
	if c.LastWirelessEnabled != enabled {
		c.LastWirelessEnabled = enabled
		c.save()
	}
}
func (c *config) setLastWwanEnabled(enabled bool) {
	if c.LastWwanEnabled != enabled {
		c.LastWwanEnabled = enabled
		c.save()
	}
}
func (c *config) setLastWiredEnabled(enabled bool) {
	if c.LastWiredEnabled != enabled {
		c.LastWiredEnabled = enabled
		c.save()
	}
}
func (c *config) setLastVpnEnabled(enabled bool) {
	if c.LastVpnEnabled != enabled {
		c.LastVpnEnabled = enabled
		c.save()
	}
}

func (c *config) setWiredEnabled(enabled bool) {
	if c.WiredEnabled != enabled {
		c.LastWiredEnabled = c.WiredEnabled
		c.WiredEnabled = enabled
		c.save()
	}
}
func (c *config) setVpnEnabled(enabled bool) {
	if c.VpnEnabled != enabled {
		c.LastVpnEnabled = c.VpnEnabled
		c.VpnEnabled = enabled
		c.save()
	}
}

func (c *config) isDeviceConfigExists(devId string) (ok bool) {
	_, ok = c.Devices[devId]
	return
}
func (c *config) getDeviceConfig(devId string) (d *deviceConfig, err error) {
	if !c.isDeviceConfigExists(devId) {
		err = fmt.Errorf("device config for %s not exists", devId)
		logger.Error(err)
		return
	}
	d, _ = c.Devices[devId]
	return
}
func (c *config) addDeviceConfig(devPath dbus.ObjectPath) {
	devId, err := nmGeneralGetDeviceIdentifier(devPath)
	if err != nil {
		return
	}
	if !c.isDeviceConfigExists(devId) {
		devConfig := newDeviceConfig()
		devConfig.lastConnectionUuid, _ = nmGetDeviceActiveConnectionUuid(devPath)
		c.Devices[devId] = devConfig
	}
	c.save()
}
func (c *config) removeDeviceConfig(devId string) {
	if !c.isDeviceConfigExists(devId) {
		logger.Errorf("device config for %s not exists", devId)
	}
	delete(c.Devices, devId)
	c.save()
}

func (c *config) setAllDeviceLastEnabled(enabled bool) {
	for _, devConfig := range c.Devices {
		devConfig.LastEnabled = enabled
	}
	// TODO vpn
	c.save()
}
func (c *config) setDeviceLastConnectionUuid(devPath dbus.ObjectPath, uuid string) {
	devId, err := nmGeneralGetDeviceIdentifier(devPath)
	if err != nil {
		return
	}
	if !c.isDeviceConfigExists(devId) {
		logger.Errorf("device config for %s not exists", devId)
	}
	devConfig, _ := c.Devices[devId]
	if devConfig.lastConnectionUuid != uuid {
		devConfig.lastConnectionUuid = uuid
		c.save()
	}
}

func (c *config) clearConnection(uuid string) {
	for _, devConfig := range c.Devices {
		if devConfig.lastConnectionUuid == uuid {
			devConfig.lastConnectionUuid = ""
		}
	}
	// TODO vpn connections
	c.save()
}

func (m *Manager) IsDeviceEnabled(devPath dbus.ObjectPath) (enabled bool, err error) {
	devId, err := nmGeneralGetDeviceIdentifier(devPath)
	if err != nil {
		enabled = true // return true as default
		return
	}
	devConfig, err := m.config.getDeviceConfig(devId)
	if err != nil {
		return
	}
	enabled = devConfig.Enabled
	return
}

func (m *Manager) EnableDevice(devPath dbus.ObjectPath, enabled bool) (err error) {
	if enabled && m.trunOnGlobalDeviceSwitchIfNeed(devPath) {
		return
	}

	devId, err := nmGeneralGetDeviceIdentifier(devPath)
	if err != nil {
		return
	}
	devConfig, err := m.config.getDeviceConfig(devId)
	if err != nil {
		return
	}

	devConfig.LastEnabled = devConfig.Enabled
	devConfig.Enabled = enabled
	if enabled {
		// active last connection if device is disconnected
		if len(devConfig.lastConnectionUuid) > 0 {
			if _, err := nmGetConnectionByUuid(devConfig.lastConnectionUuid); err == nil {
				devState, err := nmGetDeviceState(devPath)
				if err == nil {
					if !isDeviceActivated(devState) {
						err = m.ActivateConnection(devConfig.lastConnectionUuid, devPath)
					}
				}
			}
		}
	} else {
		err = m.DisconnectDevice(devPath)
	}

	// send signal
	if m.DeviceEnabled != nil {
		m.DeviceEnabled(string(devPath), enabled)
	}

	m.config.save()
	return
}

func (m *Manager) restoreDeviceState(devPath dbus.ObjectPath) (err error) {
	devId, err := nmGeneralGetDeviceIdentifier(devPath)
	if err != nil {
		return
	}
	devConfig, err := m.config.getDeviceConfig(devId)
	if err != nil {
		return
	}
	err = m.EnableDevice(devPath, devConfig.LastEnabled)
	return
}

func (m *Manager) trunOnGlobalDeviceSwitchIfNeed(devPath dbus.ObjectPath) (need bool) {
	// if global device switch is off, turn it on, and only keep
	// current device alive
	nmDev, err := nmNewDevice(devPath)
	if err != nil {
		return
	}
	devType := nmDev.DeviceType.Get()

	switch devType {
	case NM_DEVICE_TYPE_ETHERNET:
		need = (m.WiredEnabled == false)
	case NM_DEVICE_TYPE_WIFI:
		need = (m.WirelessEnabled == false)
	case NM_DEVICE_TYPE_MODEM:
		need = (m.WwanEnabled == false)
	}
	if !need {
		return
	}

	devId, err := nmGeneralGetDeviceIdentifier(devPath)
	if err != nil {
		return
	}
	devConfig, err := m.config.getDeviceConfig(devId)
	if err != nil {
		return
	}
	m.config.setAllDeviceLastEnabled(false)
	devConfig.LastEnabled = true
	switch devType {
	case NM_DEVICE_TYPE_ETHERNET:
		m.setWiredEnabled(true)
	case NM_DEVICE_TYPE_WIFI:
		m.setWirelessEnabled(true)
	case NM_DEVICE_TYPE_MODEM:
		m.setWwanEnabled(true)
	}
	return
}

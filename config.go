package main

import (
	"fmt"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/godbus/dbus"
	"github.com/mitchellh/go-homedir"
	"github.com/sqp/pulseaudio"
)

// config is a stuct with all config values. See `runtime/config/config.toml`
// for more information about these values.
var config struct {
	Sinks []sinkInfo
}

type sinkInfo struct {
	Sink  dbus.ObjectPath
	DSP   dbus.ObjectPath
	Label string
}

// parseConfig parses a toml config.
func parseConfig(c *pulseaudio.Client) error {
	hd, err := homedir.Dir()
	if err != nil {
		return err
	}

	if _, err = toml.DecodeFile(filepath.Join(hd, ".sps", "config.toml"),
		&config); err != nil {
		return fmt.Errorf("config %s: %s", filepath.Join(hd, ".sps",
			"config.toml"), err)
	}

	sl, err := c.Core().ListPath("Sinks")
	if err != nil {
		return err
	}

	for i, ci := range config.Sinks {
		for _, s := range sl {
			sn, err := c.Device(s).String("Name")
			if err != nil {
				return err
			}

			switch dbus.ObjectPath(sn) {
			case ci.Sink:
				config.Sinks[i].Sink = s
			case ci.DSP:
				config.Sinks[i].DSP = s
			}
		}
	}

	return nil
}

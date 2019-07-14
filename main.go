package main

import (
	"fmt"
	"log"
	"os"

	"github.com/fatih/color"
	"github.com/go2c/optparse"
	"github.com/godbus/dbus"
	"github.com/sqp/pulseaudio"
)

// initPulse creates a PulseAudio client and optionally loads the D-Bus module.
func initPulse() (*pulseaudio.Client, error) {
	ml, err := pulseaudio.ModuleIsLoaded()
	if err != nil {
		return nil, err
	}
	if !ml {
		if err = pulseaudio.LoadModule(); err != nil {
			return nil, err
		}
	}

	return pulseaudio.New()
}

// getFallbackSink returns the fallbacksink, this is the active sink.
func getFallbackSink(c *pulseaudio.Client) (dbus.ObjectPath, error) {
	fs, err := c.Core().ObjectPath("FallbackSink")
	if err != nil {
		return "", err
	}
	if fs == "" {
		return "", fmt.Errorf("no fallback sink found")
	}

	return fs, nil
}

// getConfigSink returns the sink as defined in the config, the active parameter
// determines if the active or the inactive sink should return.
func getConfigSink(c *pulseaudio.Client, active bool) (dbus.ObjectPath, error) {
	fs, err := getFallbackSink(c)
	if err != nil {
		return "", err
	}
	fsn, err := c.Device(fs).String("Name")
	if err != nil {
		return "", err
	}

	sl, err := c.Core().ListPath("Sinks")
	if err != nil {
		return "", err
	}

	for _, csn := range config.Sinks {
		if active {
			if csn == fsn {
				return fs, nil
			}
		} else {
			if csn != fsn {
				for _, s := range sl {
					sn, err := c.Device(s).String("Name")
					if err != nil {
						return "", err
					}

					if csn == sn {
						return s, nil
					}
				}
			}
		}
	}

	return "", fmt.Errorf("could not find sink from `config.toml`")
}

// getDSPSink returns the DPS sink, if the `d` parameter is empty it will return
// without error.
func getDSPSink(c *pulseaudio.Client, d string) (dbus.ObjectPath, error) {
	if d == "" {
		return "", nil
	}

	sl, err := c.Core().ListPath("Sinks")
	if err != nil {
		return "", err
	}

	for _, s := range sl {
		sn, err := c.Device(s).String("Name")
		if err != nil {
			return "", err
		}

		if sn == d {
			return s, nil
		}
	}

	return "", fmt.Errorf("could not find sink `%s`", d)
}

// switchSink switches the active sink to the given sink.
func switchSink(c *pulseaudio.Client, s dbus.ObjectPath, ds dbus.
	ObjectPath) error {
	if err := c.Core().Set("FallbackSink", s); err != nil {
		return err
	}

	psl, err := c.Core().ListPath("PlaybackStreams")
	if err != nil {
		return err
	}

	for _, ps := range psl {
		sm := s

		if ds > "" {
			psd, err := c.Stream(ps).String("Driver")
			if err != nil {
				return err
			}
			dd, err := c.Device(ds).String("Driver")
			if err != nil {
				return err
			}

			if psd != dd {
				sm = ds
			}
		}

		if err := c.Stream(ps).Call("org.PulseAudio.Core1.Stream.Move", 0, sm).
			Err; err != nil {
			return err
		}
	}

	return nil
}

func main() {
	// Initialize the config.
	if err := parseConfig(); err != nil {
		log.Fatal(err)
	}

	// Define valid arguments.
	argc := optparse.Bool("check", 'c', false)
	argh := optparse.Bool("help", 'h', false)

	// Parse arguments.
	_, err := optparse.Parse()
	if err != nil {
		fmt.Fprintln(os.Stderr,
			"Invaild argument, use -h for a list of arguments!")
		os.Exit(1)
	}

	// Print help.
	if *argh {
		fmt.Println("Usage: sps [arguments] ")
		fmt.Println("")
		fmt.Println("arguments:")
		fmt.Println("  -c,   --check           returns current fallback sink")
		fmt.Println("  -h,   --help            print help and exit")
		os.Exit(0)
	}

	// Initialize PulseAudio.
	c, err := initPulse()
	if err != nil {
		log.Fatal(err)
	}

	// Returns the current fallback sink.
	if *argc {
		// Get the active sink.
		s, err := getConfigSink(c, true)
		if err != nil {
			log.Fatal(err)
		}

		// Get sink properties, we need this for the label.
		pl, err := c.Device(s).MapString("PropertyList")
		if err != nil {
			log.Fatal(err)
		}

		fmt.Printf("Now playing via %s.\n", color.HiYellowString(
			pl["sps.label"]))
		return
	}

	// Get what sink we should switch to.
	s, err := getConfigSink(c, false)
	if err != nil {
		log.Fatal(err)
	}

	// Get sink properties, we need this for the label and possible DSP.
	pl, err := c.Device(s).MapString("PropertyList")
	if err != nil {
		log.Fatal(err)
	}

	// Get the DSP sink (if there is one).
	ds, err := getDSPSink(c, pl["sps.dsp"])
	if err != nil {
		log.Fatal(err)
	}

	// Move to previously determined sink(s).
	if err := switchSink(c, s, ds); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Now playing via %s.\n", color.HiYellowString(
		pl["sps.label"]))
}

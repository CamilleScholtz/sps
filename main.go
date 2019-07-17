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
func getConfigSink(c *pulseaudio.Client, active bool) (sinkInfo, error) {
	fs, err := getFallbackSink(c)
	if err != nil {
		return sinkInfo{}, err
	}

	sl, err := c.Core().ListPath("Sinks")
	if err != nil {
		return sinkInfo{}, err
	}

	for _, si := range config.Sinks {
		if active {
			if si.Sink == fs {
				return si, nil
			}
		} else {
			if si.Sink != fs {
				for _, s := range sl {
					if si.Sink == s {
						return si, nil
					}
				}
			}
		}
	}

	return sinkInfo{}, fmt.Errorf("could not find sink from `config.toml`")
}

// switchSink switches the active sink to the given sink.
func switchSink(c *pulseaudio.Client, si sinkInfo) error {
	if err := c.Core().Set("FallbackSink", si.Sink); err != nil {
		return err
	}

	psl, err := c.Core().ListPath("PlaybackStreams")
	if err != nil {
		return err
	}

	for _, ps := range psl {
		s := si.Sink

		if si.DSP > "" {
			psd, err := c.Stream(ps).String("Driver")
			if err != nil {
				return err
			}
			dd, err := c.Device(si.DSP).String("Driver")
			if err != nil {
				return err
			}

			if psd != dd {
				s = si.DSP
			}
		}

		if err := c.Stream(ps).Call("org.PulseAudio.Core1.Stream.Move", 0, s).
			Err; err != nil {
			return err
		}
	}

	return nil
}

func main() {
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

	// Initialize the config.
	if err := parseConfig(c); err != nil {
		log.Fatal(err)
	}

	// Returns the current fallback sink.
	if *argc {
		// Get the active sink.
		si, err := getConfigSink(c, true)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Printf("Now playing via %s.\n", color.HiYellowString(si.Label))
		return
	}

	// Get what sink we should switch to.
	si, err := getConfigSink(c, false)
	if err != nil {
		log.Fatal(err)
	}

	// Move to previously determined sink(s).
	if err := switchSink(c, si); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Now playing via %s.\n", color.HiYellowString(si.Label))
}

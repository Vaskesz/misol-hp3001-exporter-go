package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	port, interval, useSerial, errorLog, clockFreq, err := parseArgs()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(2)
	}
	if interval <= 0 {
		interval = 30
	}

	//printUsage()

	if err := initLogger(errorLog); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to open log file '%s': %v\n", errorLog, err)
		os.Exit(2)
	}
	defer closeLogger()

	registerBuildInfo()

	logInfo("Version: %s", versionString())

	if errorLog != "" {
		if abs, err := filepath.Abs(errorLog); err == nil {
			logInfo("Logging enabled: %s", abs)
		}
	}

	labelMode := "index (1..N)"
	if useSerial {
		labelMode = "serial number"
	}
	logInfo("Device label: %s", labelMode)

	if clockFreq > 0 {
		logInfo("Clock update: every %d polls (and on initial appearance)", clockFreq)
	} else {
		logInfo("Clock update: only during the initial poll and when the device reappears")
	}

	http.Handle("/metrics", promhttp.Handler())
	go func() {
		addr := fmt.Sprintf(":%d", port)
		logInfo("Prometheus exporter started at: http://localhost:%d/metrics", port)
		if err := http.ListenAndServe(addr, nil); err != nil {
			logError("HTTP server error: %v", err)
			os.Exit(1)
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go pollLoop(ctx, interval, useSerial, clockFreq)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh
	logInfo("Shutting down...")
	cancel()
	time.Sleep(500 * time.Millisecond)
}

// parseArgs parses command-line arguments, supporting both flags and
// positional port/interval parameters in any order.
func parseArgs() (port int, interval int, useSerial bool, errorLog string, clockFreq int, err error) {
	port = 9100
	interval = 30

	args := os.Args[1:]
	var positional []string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-h", "--help":
			printUsage()
			os.Exit(0)
		case "-v", "--version":
			fmt.Println(versionString())
			os.Exit(0)
		case "--use-serial-number":
			useSerial = true
		case "--error-log":
			if i+1 >= len(args) {
				return 0, 0, false, "", 0, fmt.Errorf("--error-log requires a file path")
			}
			i++
			errorLog = args[i]
		case "--update-clock-freq":
			if i+1 >= len(args) {
				return 0, 0, false, "", 0, fmt.Errorf("--update-clock-freq requires an integer")
			}
			i++
			v, e := strconv.Atoi(args[i])
			if e != nil || v <= 0 {
				return 0, 0, false, "", 0, fmt.Errorf(
					"--update-clock-freq requires a positive integer, got '%s'", args[i])
			}
			clockFreq = v
		default:
			positional = append(positional, args[i])
		}
	}

	if len(positional) > 0 {
		p, e := strconv.Atoi(positional[0])
		if e != nil {
			return 0, 0, false, "", 0, fmt.Errorf("port must be an integer, got '%s'", positional[0])
		}
		port = p
	}
	if len(positional) > 1 {
		v, e := strconv.Atoi(positional[1])
		if e != nil {
			return 0, 0, false, "", 0, fmt.Errorf("interval must be an integer, got '%s'", positional[1])
		}
		interval = v
	}
	return
}

func pollLoop(ctx context.Context, intervalSec int, useSerialNumber bool, clockFreq int) {
	deviceStates := make(map[string]int)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		devices, err := openDevices()
		if err != nil {
			logWarn("enumeration failed: %v", err)
		} else if len(devices) == 0 {
			logWarn("HP3001 devices not found")
			deviceStates = make(map[string]int)
		} else {
			logInfo("Polling %d device(s)...", len(devices))
			currentKeys := make(map[string]bool)

			for i, dev := range devices {
				deviceLabel := fmt.Sprintf("%d", i+1)
				if useSerialNumber && dev.serial != "" {
					deviceLabel = strings.ToLower(dev.serial)
				}

				key := dev.Key(i)
				currentKeys[key] = true

				counter := deviceStates[key]
				counter++
				deviceStates[key] = counter

				needUpdate := counter == 1
				if !needUpdate && clockFreq > 0 {
					needUpdate = ((counter - 1) % clockFreq) == 0
				}
				if needUpdate {
					updateCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
					err := dev.UpdateClock(updateCtx, time.Now())
					cancel()
					if err != nil {
						logError("Device '%s': clock update error — %v", deviceLabel, err)
					} else {
						logInfo("Device '%s': clocks updated to %s",
							deviceLabel, time.Now().Format("2006-01-02 15:04:05"))
					}
				}

				pollCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
				temps, hums, err := dev.Poll(pollCtx)
				cancel()
				if err != nil {
					logError("Device '%s': polling error — %v", deviceLabel, err)
					continue
				}

				for ch := 0; ch < channelCount; ch++ {
					channelLabel := fmt.Sprintf("%d", ch+1)
					if temps[ch] != nil {
						tempGauge.WithLabelValues(deviceLabel, channelLabel).Set(*temps[ch])
					}
					if hums[ch] != nil {
						humGauge.WithLabelValues(deviceLabel, channelLabel).Set(float64(*hums[ch]))
					}
				}
			}

			// Remove devices that are no longer present.
			for key := range deviceStates {
				if !currentKeys[key] {
					delete(deviceStates, key)
					logWarn("Device '%s' lost — reset state", key)
				}
			}

			for _, dev := range devices {
				_ = dev.Close()
			}
		}

		sleepWithPresenceCheck(ctx, intervalSec, deviceStates)
	}
}

// sleepWithPresenceCheck waits intervalSec seconds, but wakes up once per
// second to detect devices that have appeared or disappeared. Newly appeared
// devices are announced and their clocks are synchronized immediately.
func sleepWithPresenceCheck(ctx context.Context, intervalSec int, deviceStates map[string]int) {
	deadline := time.Now().Add(time.Duration(intervalSec) * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return
		case <-time.After(1 * time.Second):
			present, err := presentKeys()
			if err != nil {
				// Enumeration may occasionally fail (USB transients) —
				// not fatal; we'll try again in a second.
				continue
			}

			// Detect disappeared devices.
			for key := range deviceStates {
				if _, ok := present[key]; !ok {
					delete(deviceStates, key)
					logWarn("Device '%s' lost — reset state", key)
				}
			}

			// Detect newly appeared devices and sync their clocks immediately.
			for key, path := range present {
				if _, exists := deviceStates[key]; exists {
					continue
				}
				dev, err := openDeviceByPath(path)
				if err != nil {
					logWarn("Device '%s': open failed — %v", key, err)
					continue
				}
				updateCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
				err = dev.UpdateClock(updateCtx, time.Now())
				cancel()
				_ = dev.Close()
				if err != nil {
					logError("Device '%s': clock update error on appearance — %v", key, err)
					// Do not register the device yet; retry on the next tick.
					continue
				}
				logInfo("Device '%s' appeared — clocks updated to %s",
					key, time.Now().Format("2006-01-02 15:04:05"))
				// Mark as present with counter=1 so pollLoop does not
				// update the clock a second time on the next cycle.
				deviceStates[key] = 1
			}
		}
	}
}

// printUsage writes the command-line usage text to stdout. The program
// name is taken from os.Args[0] (base name only, without directory).
func printUsage() {
	progName := filepath.Base(os.Args[0])
	fmt.Printf(`Usage:
%s [port [interval]] [--use-serial-number] [--error-log path/to/log.file] [--update-clock-freq X]
%s -h | --help     -> Show this help and exit.
%s -v | --version  -> Show version information and exit.
[port [interval]] -> If an interval other than the default (30 seconds) is required, you must also specify the port in the command line (the default is 9100).
--use-serial-number -> Uses the device's serial number (if available) as the device identifier in metrics; otherwise, an index is used.
--error-log path/to/log.file -> Writes logs to a file; the path can be relative or absolute.
--update-clock-freq X -> If this parameter is specified (where X is a positive integer), the time on the base unit will be updated every X polling cycles.
      For example, if X=5 and the default polling interval is used, the time on the base unit will be updated every 2.5 minutes, as well as during the initial poll of the device.
      If this parameter is not set, the time will be updated during the initial poll of the device, as well as whenever the device reappears after having been offline.
`, progName, progName, progName)
}

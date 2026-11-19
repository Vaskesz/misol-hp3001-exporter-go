package main

import (
    "context"
    "fmt"
    "strings"
    "time"

    "github.com/telesma-app/hid"
)

const (
    vid          = 0x0483
    pid          = 0x5750
    channelCount = 8
)

type HP3001Device struct {
    dev    *hid.Device
    serial string
    path   string
}

// descriptorString safely extracts a string value from a hid.Descriptor[string].
// It returns an empty string if the value is nil or an error occurred.
func descriptorString(d hid.Descriptor[string]) string {
    if d.Value != nil {
        return *d.Value
    }
    return ""
}

// openDevices enumerates all connected HP3001 devices and opens them.
func openDevices() ([]*HP3001Device, error) {
    var result []*HP3001Device
    for info, err := range hid.Enumerate(
        hid.WithVendorID(vid),
        hid.WithProductID(pid),
    ) {
        if err != nil {
            return nil, err
        }
        if info == nil {
            continue
        }
        dev, err := hid.OpenPath(info.Path)
        if err != nil {
            logWarn("failed to open device %s: %v", info.Path, err)
            continue
        }
        result = append(result, &HP3001Device{
            dev:    dev,
            serial: descriptorString(info.SerialNbr),
            path:   info.Path,
        })
    }
    return result, nil
}

func (d *HP3001Device) Close() error {
    return d.dev.Close()
}

// Key returns a stable device identifier: serial number if available,
// otherwise the device path, otherwise the given index.
func (d *HP3001Device) Key(index int) string {
    if d.serial != "" {
        return "SN:" + strings.ToLower(d.serial)
    }
    if d.path != "" {
        return "PATH:" + d.path
    }
    return fmt.Sprintf("IDX:%d", index)
}

// Poll performs one polling cycle and returns temperature and humidity
// values for 8 channels. Nil values indicate missing sensor readings.
func (d *HP3001Device) Poll(ctx context.Context) ([]*float64, []*uint8, error) {
    req := []byte{0x00, 0x7B, 0x03, 0x40, 0x7D}
    if _, err := d.dev.Write(ctx, req); err != nil {
        return nil, nil, fmt.Errorf("write: %w", err)
    }

    buf := make([]byte, 65)
    n, err := d.dev.Read(ctx, buf)
    if err != nil {
        return nil, nil, fmt.Errorf("read: %w", err)
    }
    if n < 27 {
        return nil, nil, fmt.Errorf("incorrect response length: %d", n)
    }
    // telesma-app/hid returns the raw report data without a leading
    // Report ID byte, so the buffer starts directly with the 0x7B
    // start flag. The fixed trailer (0x40, 0x7D) is at indices 25 and 26.
    if buf[0] != 0x7B || buf[25] != 0x40 || buf[26] != 0x7D {
        return nil, nil, fmt.Errorf("invalid response format")
    }

    temps := make([]*float64, channelCount)
    hums := make([]*uint8, channelCount)

    for i := 0; i < channelCount; i++ {
        offset := 1 + i*3

        raw := int(buf[offset])<<8 | int(buf[offset+1])
        if raw >= 0x8000 {
            raw -= 0x10000
        }
        if raw == 0x7FFF {
            temps[i] = nil
        } else {
            v := float64(raw) / 10.0
            temps[i] = &v
        }

        h := buf[offset+2]
        if h == 0xFF {
            hums[i] = nil
        } else {
            hv := h
            hums[i] = &hv
        }
    }

    return temps, hums, nil
}

// UpdateClock synchronizes the device clock with the given time.
func (d *HP3001Device) UpdateClock(ctx context.Context, t time.Time) error {
    year := t.Year()
    packet := []byte{
        0x00,
        0x7B,
        0x30,
        byte((year >> 8) & 0xFF),
        byte(year & 0xFF),
        byte(t.Month()),
        byte(t.Day()),
        byte(t.Hour()),
        byte(t.Minute()),
        byte(t.Second()),
        0x02,
        0x40,
        0x7D,
    }

    if _, err := d.dev.Write(ctx, packet); err != nil {
        return err
    }

    // Best-effort read of the response with a short timeout to avoid
    // leaving stale data in the buffer for the next Poll.
    shortCtx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
    defer cancel()
    buf := make([]byte, 65)
    _, _ = d.dev.Read(shortCtx, buf)
    return nil
}

// presentKeys returns a map of device keys to device paths for all
// currently visible HP3001 devices. The keys match the format produced
// by Key(). This function does not open the devices, so it is cheap.
func presentKeys() (map[string]string, error) {
    result := make(map[string]string)
    idx := 0

    for info, err := range hid.Enumerate(
        hid.WithVendorID(vid),
        hid.WithProductID(pid),
    ) {
        if err != nil {
            return nil, err
        }
        if info == nil {
            continue
        }

        var key string
        if sn := descriptorString(info.SerialNbr); sn != "" {
            key = "SN:" + strings.ToLower(sn)
        } else if info.Path != "" {
            key = "PATH:" + info.Path
        } else {
            key = fmt.Sprintf("IDX:%d", idx)
        }
        result[key] = info.Path
        idx++
    }
    return result, nil
}

// openDeviceByPath opens a single HP3001 device by its HID path.
// The serial number is not retrieved; use Key() or the path for
// identification. Callers must close the returned device.
func openDeviceByPath(path string) (*HP3001Device, error) {
    dev, err := hid.OpenPath(path)
    if err != nil {
        return nil, err
    }
    return &HP3001Device{dev: dev, path: path}, nil
}

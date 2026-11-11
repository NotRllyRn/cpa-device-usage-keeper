package web

import (
	"embed"
	"io/fs"
)

// DeviceAssets is a separate build: upstream dist and application stay untouched.
//
//go:embed all:device-dist
var deviceAssets embed.FS

func DeviceAssets() fs.FS {
	assets, err := fs.Sub(deviceAssets, "device-dist")
	if err != nil {
		panic(err)
	}
	return assets
}

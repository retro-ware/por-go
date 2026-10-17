//go:build !js

package main

import "os"

var saveFile, settingsFile = "blobber.sav", "settings.json"

func writeSave(b []byte) error  { return os.WriteFile(saveFile, b, 0o644) }
func readSave() ([]byte, error) { return os.ReadFile(saveFile) }

func writeSettings(b []byte) error  { return os.WriteFile(settingsFile, b, 0o644) }
func readSettings() ([]byte, error) { return os.ReadFile(settingsFile) }

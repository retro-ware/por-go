//go:build js

package main

import (
	"errors"
	"syscall/js"
)

func writeSave(b []byte) error      { return store("blobber.sav", b) }
func readSave() ([]byte, error)     { return load("blobber.sav") }
func writeSettings(b []byte) error  { return store("blobber.settings", b) }
func readSettings() ([]byte, error) { return load("blobber.settings") }

func store(key string, b []byte) error {
	js.Global().Get("localStorage").Call("setItem", key, string(b))
	return nil
}

func load(key string) ([]byte, error) {
	v := js.Global().Get("localStorage").Call("getItem", key)
	if v.IsNull() {
		return nil, errors.New("nothing stored")
	}
	return []byte(v.String()), nil
}

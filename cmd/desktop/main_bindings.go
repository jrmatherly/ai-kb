//go:build bindings

package main

import (
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
)

// The Wails "generate bindings" step compiles this file alone with -tags bindings, without
// starting Gin or the database, so it does not need a local Postgres.
func main() {
	app := NewApp()
	_ = wails.Run(&options.App{
		Title: "AI KB Lite",
		Bind:  []interface{}{app},
	})
}

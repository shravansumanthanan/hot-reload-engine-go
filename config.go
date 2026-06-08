package main

// This file re-exports types and helpers from internal/config so that
// the package main wiring code can use short names without an import alias.
// All logic lives in internal/config.

import "github.com/shravansumanthanan/hot-reload-engine-go/internal/config"

type Config = config.Config

var (
	LoadConfig        = config.LoadConfig
	WriteExampleConfig = config.WriteExampleConfig
)

const (
	defaultDebounceDelay        = config.DefaultDebounceDelay
	defaultReloadBroadcastDelay = config.DefaultReloadBroadcastDelay
	defaultWatchExtensions      = config.DefaultWatchExtensions
	defaultRootPath             = config.DefaultRootPath
)

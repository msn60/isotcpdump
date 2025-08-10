package zrlogger

import (
	"strings"
	"time"

	"github.com/msn60/isotcpdump/config"
	"github.com/rs/zerolog"
)

func OptionsFromConfig(cfg *config.Config) *Options {
	// set log level
	var lvl *zerolog.Level
	if l, err := zerolog.ParseLevel(strings.ToLower(cfg.Log.Metadata.Level)); err == nil {
		lvl = &l
	}

	// check env to enable console logger
	env := strings.TrimSpace(strings.ToLower(cfg.EnvVars["APP_ENV"]))
	var dev bool
	switch env {
	case "prod", "production":
		dev = false
	case "dev", "development":
		dev = true
	default:
		dev = false
	}

	// set apply to global
	var apply bool
	switch cfg.Log.Metadata.ApplyToGlobal {
	case true:
		apply = true
	case false:
		apply = false
	default:
		apply = false
	}

	// set file path
	filePath := ""
	if cfg.Log.File.Path != "" {
		filePath = "logs/app.log"
	} else {
		filePath = cfg.Log.File.Path
	}

	sizeMB := cfg.Log.File.MaxSizeMB
	if sizeMB <= 0 {
		sizeMB = 50
	}

	maxBackups := cfg.Log.File.MaxBackups
	if maxBackups <= 0 {
		maxBackups = 10
	}

	maxAgeDays := 14
	if cfg.Log.File.MaxAgeDays > 0 {
		maxAgeDays = cfg.Log.File.MaxAgeDays
	}

	// time & duration
	timeFormat := cfg.Log.Time.FieldFormat
	if timeFormat == "" || strings.EqualFold(timeFormat, "RFC3339Nano") {
		timeFormat = time.RFC3339Nano
	}
	var durUnit time.Duration
	switch strings.ToLower(cfg.Log.Time.DurationFieldUnit) {
	case "ns":
		durUnit = time.Nanosecond
	case "us", "µs":
		durUnit = time.Microsecond
	case "s":
		durUnit = time.Second
	default: // "ms" یا خالی
		durUnit = time.Millisecond
	}

	// sampler
	var burstPeriod time.Duration
	if d := strings.TrimSpace(cfg.Log.Sampler.BurstPeriod); d != "" {
		if parsed, err := time.ParseDuration(d); err == nil {
			burstPeriod = parsed
		}
	}
	return &Options{
		Env:           env,
		Service:       cfg.Log.Metadata.Service,
		Level:         lvl,
		ApplyToGlobal: apply,

		// Console
		ConsoleEnable:        dev,
		ConsolePipe:          cfg.Log.Console.Pipe || dev,
		ConsoleFieldsExclude: cfg.Log.Console.FieldsExclude,

		// File
		FileEnable:     cfg.Log.File.Enable || true,
		FilePath:       filePath,
		FileMaxSizeMB:  sizeMB,
		FileMaxBackups: maxBackups,
		FileMaxAgeDays: maxAgeDays,
		FileCompress:   cfg.Log.File.Compress,

		// Time
		TimeFieldFormat:      timeFormat,
		DurationFieldUnit:    durUnit,
		DurationFieldInteger: cfg.Log.Time.DurationFieldInteger,

		// system
		EnableCaller:  cfg.Log.System.EnableCaller && dev, // dev: on, prod: off
		EnablePIDHost: cfg.Log.System.EnablePIDHost,

		// Sampling
		EnableSampling:     cfg.Log.Sampler.Enable,
		SamplerBurst:       cfg.Log.Sampler.Burst,
		SamplerBurstPeriod: burstPeriod,
		SamplerNextEveryN:  cfg.Log.Sampler.NextEveryN,
		SamplerBasicN:      cfg.Log.Sampler.BasicN,
	}
}

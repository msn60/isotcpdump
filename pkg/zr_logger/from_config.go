package zrlogger

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/msn60/isotcpdump/config"
	"github.com/rs/zerolog"
)

func OptionsFromConfig(cfg *config.Config) *Options {
	var lvl *zerolog.Level
	if l, err := zerolog.ParseLevel(strings.ToLower(cfg.Log.Metadata.Level)); err == nil {
		lvl = &l
	}
	sizeKB := cfg.Log.Rotation.Size
	if sizeKB <= 0 {
		sizeKB = 102400
	} // 100MB KB
	sizeMB := sizeKB / 1024
	if sizeMB < 1 {
		sizeMB = 1
	}
	maxBackups := cfg.Log.Rotation.Count
	if maxBackups <= 0 {
		maxBackups = 10
	}

	var maxAgeDays int
	switch strings.ToLower(cfg.Log.Rotation.Interval) {
	case "daily":
		maxAgeDays = 1
	case "weekly":
		maxAgeDays = 7
	case "hourly":
		maxAgeDays = 0 // lumberjack ساعتی ندارد
	}

	filePath := "logs/app.log"
	if cfg.Log.Metadata.Path != "" {
		filePath = filepath.Join(cfg.Log.Metadata.Path, "app.log")
	}

	env := strings.ToLower(cfg.EnvVars["APP_ENV"])
	dev := env == "development"

	return &Options{
		Env:           env,
		Service:       cfg.App.Name,
		Level:         lvl,
		ApplyToGlobal: true,

		ConsoleEnable:  dev,  // dev: کنسول
		ConsolePipe:    true, // pipe format در کنسول
		FileEnable:     true, // همیشه فایل
		FilePath:       filePath,
		FileMaxSizeMB:  sizeMB,
		FileMaxBackups: maxBackups,
		FileMaxAgeDays: maxAgeDays,
		FileCompress:   true,

		TimeFieldFormat:      time.RFC3339Nano,
		DurationFieldUnit:    time.Millisecond,
		DurationFieldInteger: true,
		EnableCaller:         true,
		EnablePIDHost:        true,
	}
}

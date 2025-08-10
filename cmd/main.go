package main

import (
	"fmt"
	"log"
	"os"

	"github.com/msn60/isotcpdump/config"
	zrlogger "github.com/msn60/isotcpdump/pkg/zr_logger"
	"github.com/rs/zerolog"
)

func main() {

	// TODO: equal config.yaml & config.toml
	// TODO: test loading config.toml
	// TODO: test config.yaml with bad inputs & test all cases
	// TODO: check sampler & test it
	// TODO: check log rotation

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config load error: %v", err)
	}

	// ساخت Options از روی کانفیگ
	opts := zrlogger.OptionsFromConfig(cfg)

	// اگر Singleton می‌خوای:
	if err := zrlogger.Init(opts); err != nil {
		log.Fatalf("logger init error: %v", err)
	}
	console := zrlogger.Console()
	file := zrlogger.File()

	// ساخت اپ و تزریق لاگر
	app := config.NewApp(cfg).WithLogger(console, file)
	zrlogger.SetLevel(zerolog.DebugLevel)

	// نمونه استفاده
	file.Info().Str("version", cfg.App.Version).Msg("application started")
	// console.Debug().Str("version", cfg.App.Version).Msg("application started")
	// console.Debug().RawJSON("config", []byte(cfg.Pretty())).Msg("loaded config (dev view)")
	fmt.Fprintln(os.Stdout, "STDOUT test")
	fmt.Fprintln(os.Stderr, "STDERR test")
	console.Info().Str("version", cfg.App.Version).Msg("application started")
	console.Debug().RawJSON("config", []byte(cfg.Pretty())).Msg("loaded config (dev view)")
	config.Print(cfg)
	fmt.Println(app)

}

package main

import (
	"fmt"
	"log"

	"github.com/msn60/isotcpdump/config"
	zrlogger "github.com/msn60/isotcpdump/pkg/zr_logger"
)

func main() {
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

	// نمونه استفاده
	file.Info().Str("version", cfg.App.Version).Msg("application started")
	console.Debug().Str("version", cfg.App.Version).Msg("application started")
	console.Debug().RawJSON("config", []byte(cfg.Pretty())).Msg("loaded config (dev view)")
	config.Print(cfg)
	fmt.Println(app)

}

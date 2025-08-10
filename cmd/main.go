package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/google/gopacket/tcpassembly"
	"github.com/msn60/isotcpdump/config"
	zrlogger "github.com/msn60/isotcpdump/pkg/zr_logger"
	"github.com/msn60/isotcpdump/stream"
)

func main() {

	// TODO: equal config.yaml & config.toml
	// TODO: test loading config.toml
	// TODO: test config.yaml with bad inputs & test all cases
	// TODO: check sampler & test it
	// TODO: check log rotation
	// TODO: check all of logic in zrlogger
	// TODO: gather all message

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config load error: %v", err)
	}

	opts := zrlogger.OptionsFromConfig(cfg)

	if err := zrlogger.Init(opts); err != nil {
		log.Fatalf("logger init error: %v", err)
	}
	console := zrlogger.Console()
	file := zrlogger.File()

	app := config.NewApp(cfg).WithLogger(console, file)
	app.Flogger.Info().Str("version", cfg.App.Version).Msg("application started")
	app.Clogger.Info().Str("version", cfg.App.Version).Msg("application started")
	// console.Debug().RawJSON("config", []byte(cfg.Pretty())).Msg("loaded config (dev view)")
	// config.Print(cfg)
	// fmt.Println(app)

	pcapPath := strings.TrimSpace(app.Cfg.App.PcapPath)

	if pcapPath == "" {
		app.Clogger.Fatal().Msg("pcap path is empty in config")
		os.Exit(1)
	}

	handle, err := pcap.OpenOffline(pcapPath)
	if err != nil {
		app.Clogger.Fatal().Err(err).Str("pcap_path", pcapPath).Msg("failed to open pcap file")
		os.Exit(1)
	}
	defer handle.Close()
	runWithStreams(app, handle, 1000)
}

func runWithStreams(app *config.Application, handle *pcap.Handle, maxCSVRows int) {
	// 1)
	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())

	// 2)
	agg := stream.NewAggregator(maxCSVRows)
	factory := stream.NewFactory(app.Cfg.Network.FWIP, agg) // ← فیلد IP فایروال از کانفیگ خودت

	pool := tcpassembly.NewStreamPool(factory)
	assembler := tcpassembly.NewAssembler(pool)

	// 3) counters
	var totalPackets int
	var payloadPackets int

	// 4)
	for pkt := range packetSource.Packets() {
		totalPackets++

		if pkt.NetworkLayer() == nil || pkt.TransportLayer() == nil {
			continue
		}
		tcp, ok := pkt.TransportLayer().(*layers.TCP)
		if !ok {
			continue
		}
		if len(tcp.Payload) > 0 {
			payloadPackets++
		}

		assembler.Assemble(pkt.NetworkLayer().NetworkFlow(), tcp)
	}

	// 5)
	assembler.FlushAll()
	time.Sleep(500 * time.Millisecond)

	// 6)
	resp := agg.Snapshot()

	// 7)
	// _ = writeCSV("input_packets.csv", resp.InputRows)
	// _ = writeCSV("output_packets.csv", resp.OutputRows)

	// 8) final report
	fmt.Println("✅ Processing complete")
	fmt.Println("📦 Total packets:", totalPackets)
	fmt.Println("💾 Packets with payload:", payloadPackets)
	fmt.Println("📥 Input messages:", resp.TotalInputMessages)
	fmt.Println("📤 Output messages:", resp.TotalOutputMessages)
	fmt.Println("📝 Input messages in CSV:", len(resp.InputRows))
	fmt.Println("📝 Output messages in CSV:", len(resp.OutputRows))
}

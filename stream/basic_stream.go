package stream

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/google/gopacket/tcpassembly"
	"github.com/google/gopacket/tcpassembly/tcpreader"
	"github.com/msn60/isotcpdump/config"
)

type simpleStream struct {
	r tcpreader.ReaderStream
}

func (s *simpleStream) run() {
	buf := make([]byte, 4096)
	for {
		n, err := s.r.Read(buf)
		if n > 0 {
			os.Stdout.Write(buf[:n])
		}
		if err == io.EOF {
			return
		}
		if err != nil {
			log.Printf("read error: %v", err)
		}
	}
}

type simpleFactory struct{}

func (f *simpleFactory) New(netFlow, tcpFlow gopacket.Flow) tcpassembly.Stream {
	rs := tcpreader.NewReaderStream()
	s := &simpleStream{r: rs}
	go s.run()
	return &rs
}

func PrintBytes(cfg *config.Config) {
	handle, err := pcap.OpenOffline(cfg.App.PcapPath)
	if err != nil {
		panic(err)
	}
	defer handle.Close()
	
	//create a Stream pool
	pool := tcpassembly.NewStreamPool(&simpleFactory{}) 
	assembler := tcpassembly.NewAssembler(pool)

	src := gopacket.NewPacketSource(handle, handle.LinkType())
	for pkt := range src.Packets() {
		if pkt == nil || pkt.NetworkLayer() == nil || pkt.TransportLayer() == nil {
			continue
		}
		if tcp, ok := pkt.TransportLayer().(*layers.TCP); ok {
			assembler.Assemble(pkt.NetworkLayer().NetworkFlow(), tcp)
		}
	}

	assembler.FlushAll()
	fmt.Fprintln(os.Stderr, "\nDONE")
}

package stream

import (
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/tcpassembly"
	"github.com/google/gopacket/tcpassembly/tcpreader"
)

var (
	mutex sync.Mutex
)

type IsoStreamResponse struct {
	InputRows           [][]string
	OutputRows          [][]string
	TotalInputMessages  int
	TotalOutputMessages int
}

// ---- Aggregator for all streams----

type Aggregator struct {
	mu                  sync.Mutex
	inputRows           [][]string
	outputRows          [][]string
	totalInputMessages  int
	totalOutputMessages int
	maxCSVRows          int
}

func NewAggregator(maxCSVRows int) *Aggregator {
	return &Aggregator{
		inputRows:  make([][]string, 0, maxCSVRows),
		outputRows: make([][]string, 0, maxCSVRows),
		maxCSVRows: maxCSVRows,
	}
}

func (a *Aggregator) addInputRow(row []string) {
	a.mu.Lock()
	if len(a.inputRows) < a.maxCSVRows {
		a.inputRows = append(a.inputRows, row)
	}
	a.totalInputMessages++
	a.mu.Unlock()
}

func (a *Aggregator) addOutputRow(row []string) {
	a.mu.Lock()
	if len(a.outputRows) < a.maxCSVRows {
		a.outputRows = append(a.outputRows, row)
	}
	a.totalOutputMessages++
	a.mu.Unlock()
}

func (a *Aggregator) Snapshot() *IsoStreamResponse {
	a.mu.Lock()
	defer a.mu.Unlock()

	// copy shallow for safety
	outIn := make([][]string, len(a.inputRows))
	copy(outIn, a.inputRows)
	outOut := make([][]string, len(a.outputRows))
	copy(outOut, a.outputRows)

	return &IsoStreamResponse{
		InputRows:           outIn,
		OutputRows:          outOut,
		TotalInputMessages:  a.totalInputMessages,
		TotalOutputMessages: a.totalOutputMessages,
	}
}

// ---- helpers ----

func isDigits(s string) bool {
	for _, b := range s {
		if b < '0' || b > '9' {
			return false
		}
	}
	return true
}

func isLikelyISO8583(data []byte) bool {
	if len(data) < 4 {
		return false
	}
	mti := string(data[0:4])
	valid := []string{"0200", "0210", "0220", "0230", "0800", "0810", "0820", "0830", "0840", "0100", "0110", "0120", "0130", "0400", "0410", "0420", "0430"}
	for _, v := range valid {
		if mti == v {
			return true
		}
	}
	return false
}

func extractKey(payload []byte) (string, error) {
	if len(payload) < 64 {
		return "", fmt.Errorf("payload too short")
	}
	mti := string(payload[0:4])
	pan := string(payload[38:54])
	proc := string(payload[54:60])
	return fmt.Sprintf("%s_%s_%s", mti, pan, proc), nil
}

// ---- stream types ----

type isoStream struct {
	net, transport gopacket.Flow
	reader         tcpreader.ReaderStream
	srcIP          string
	dstIP          string

	fwIP string
	agg  *Aggregator
}

func (h *isoStream) run() {
	buffer := make([]byte, 0)
	tmp := make([]byte, 4096)

	for {
		n, err := h.reader.Read(tmp)
		if n > 0 {
			buffer = append(buffer, tmp[:n]...)
			for {
				if len(buffer) < 4 {
					break
				}
				if !isDigits(string(buffer[:4])) {
					buffer = buffer[1:]
					continue
				}
				length, err := strconv.Atoi(string(buffer[:4]))
				if err != nil || length <= 0 || len(buffer) < 4+length {
					break
				}
				msg := buffer[4 : 4+length]
				buffer = buffer[4+length:]

				if !isLikelyISO8583(msg) {
					continue
				}

				key, err := extractKey(msg)
				if err != nil {
					key = "[key-error]"
				}
				timestamp := time.Now().Format(time.RFC3339Nano)
				row := []string{timestamp, key}

				// تجمیع بر اساس جهت
				if h.dstIP == h.fwIP {
					h.agg.addInputRow(row)
				} else if h.srcIP == h.fwIP {
					h.agg.addOutputRow(row)
				}
			}
		}
		if err == io.EOF {
			break
		}
		// سایر خطاها رو نادیده می‌گیریم؛ ReaderStream معمولاً با EOF تموم میشه
	}
}

// ---- factory ----

type isoFactory struct {
	fwIP string
	agg  *Aggregator
}

func NewFactory(fwIP string, agg *Aggregator) *isoFactory {
	return &isoFactory{
		fwIP: fwIP,
		agg:  agg,
	}
}

func (f *isoFactory) New(netFlow, tcpFlow gopacket.Flow) tcpassembly.Stream {
	r := tcpreader.NewReaderStream()
	src := net.IP(netFlow.Src().Raw()).String()
	dst := net.IP(netFlow.Dst().Raw()).String()
	h := &isoStream{
		net:       netFlow,
		transport: tcpFlow,
		reader:    r,
		srcIP:     src,
		dstIP:     dst,
		fwIP:      f.fwIP,
		agg:       f.agg,
	}
	go h.run()
	return &r
}

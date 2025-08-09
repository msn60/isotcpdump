package config

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

var (
	k = koanf.New(".")
)

type Application struct {
	Cfg Config
}

func NewApp(cfg Config) *Application {
	return &Application{Cfg: cfg}
}

type Config struct {
	App          App          `koanf:"app"`
	Network      Network      `koanf:"network"`
	Server       []Server     `koanf:"server"`
	Output       Output       `koanf:"output"`
	Limits       Limits       `koanf:"limits"`
	Log          Log          `koanf:"log"`
	CrossNetwork CrossNetwork `koanf:"crossnetwork"`
	EnvVars      map[string]string
}

type App struct {
	Version string `koanf:"version"`
	Name    string `koanf:"name"`
}

type Network struct {
	FWIP string `koanf:"fw_ip"`
}

type Server struct {
	Name      string `koanf:"name"`
	IP        string `koanf:"ip"`
	Ports     []int  `koanf:"ports"`
	IsEnable  bool   `koanf:"is_enable"`
	IsDefault bool   `koanf:"is_default"`
}

type Output struct {
	PacketLogPath   string `koanf:"packet_log_path"`
	IsNeedPacketLog bool   `koanf:"is_need_packet_log"`
	InputCSVPath    string `koanf:"input_csv_path"`
	OutputCSVPath   string `koanf:"output_csv_path"`
	DurationsCSV    string `koanf:"durations_csv"`
}

type Limits struct {
	MaxRecords    int `koanf:"max_records"`
	MaxPacketLogs int `koanf:"max_packet_logs"`
	LimitSize     int
}

type Log struct {
	Path     string      `koanf:"path"`
	Level    string      `koanf:"level"`
	Rotation LogRotation `koanf:"rotation"`
}

type LogRotation struct {
	Type     string `koanf:"type"`
	Size     int    `koanf:"size"`
	Count    int    `koanf:"count"`
	Interval string `koanf:"interval"`
}

type CrossNetwork struct {
	Net []CrossNet `koanf:"net"`
}

type CrossNet struct {
	Src   string `koanf:"src"`
	Dest  string `koanf:"dest"`
	OID   string `koanf:"oid"`
	Index int    `koanf:"index"`
}

func Load() (*Config, error) {

	var cfg Config

	// read .env
	envs := readDotEnvAll(".env")

	// load EnvVars except LIMIT_SIZE
	cfg.EnvVars = make(map[string]string, len(envs))
	for k, v := range envs {
		if k == "LIMIT_SIZE" {
			iv, _ := strconv.Atoi(v)
			cfg.Limits.LimitSize = iv
			continue
		}
		cfg.EnvVars[k] = v
	}

	// get type of config from .env
	fileType := strings.ToLower(envs["CONFIG_FILE_TYPE"])
	if fileType != "toml" && fileType != "yaml" {
		fileType = "yaml"
	}

	var path string

	switch fileType {
	case "toml":
		path = filepath.Join("config", "config.toml")
		if err := k.Load(file.Provider(path), toml.Parser()); err != nil {
			return nil, fmt.Errorf("config: load toml: %w", err)
		}
	default:
		path = filepath.Join("config", "config.yaml")
		if err := k.Load(file.Provider(path), yaml.Parser()); err != nil {
			return nil, fmt.Errorf("config: load yaml: %w", err)
		}
	}

	// if you need to normalize some structure
	normalizeAllListOfSingletonMaps(k,
		"log.rotation",
		// add another configs which you need
	)

	if err := k.Unmarshal("", &cfg); err != nil {
		return nil, fmt.Errorf("config: unmarshal: %w", err)
	}

	return &cfg, nil
}

func readDotEnvAll(path string) map[string]string {
	out := make(map[string]string)
	f, err := os.Open(path)
	if err != nil {
		return out
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if i := strings.Index(line, "="); i > 0 {
			k := strings.TrimSpace(line[:i])
			v := strings.TrimSpace(line[i+1:])
			out[k] = strings.Trim(v, `"'`)
		}
	}
	return out
}

func (c Config) Pretty() string {
	b, _ := json.MarshalIndent(c, "", "  ")
	return string(b)
}

func (c Config) PrettyInOneLine() string {
	lines := []string{}

	sections := map[string]interface{}{
		"app":          c.App,
		"network":      c.Network,
		"server":       c.Server,
		"output":       c.Output,
		"limits":       c.Limits,
		"log":          c.Log,
		"crossnetwork": c.CrossNetwork,
	}

	for k, v := range sections {
		b, _ := json.Marshal(v)
		lines = append(lines, fmt.Sprintf("%s: %s", k, string(b)))
	}

	return strings.Join(lines, "\n")
}

func Print(cfg *Config) {
	fmt.Println(cfg.Pretty())
	// fmt.Println(Cfg.PrettyInOneLine())
}

func normalizeListOfSingletonMaps(k *koanf.Koanf, path string) {
	v := k.Get(path)
	slice, ok := v.([]any)
	if !ok || len(slice) == 0 {
		return
	}
	m := make(map[string]any, len(slice))
	for _, it := range slice {
		if mm, ok := it.(map[string]any); ok {
			for kk, vv := range mm {
				m[kk] = vv
			}
		}
	}
	_ = k.Set(path, m)
}

func normalizeAllListOfSingletonMaps(k *koanf.Koanf, paths ...string) {
	for _, p := range paths {
		normalizeListOfSingletonMaps(k, p)
	}
}

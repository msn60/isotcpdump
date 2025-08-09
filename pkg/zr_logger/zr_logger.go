// Package zrlogger — zerolog with selecting log path
// - Singleton: Init + Get/Both/File/Console + SetLevel
// - DI: New + NewTargets
// - File: JSON + lumberjack rotation
// - Console: Pipe format  time | LEVEL | message | k=v ...
package zrlogger

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/pkgerrors"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	once          sync.Once
	consoleLogger zerolog.Logger
	fileLogger    zerolog.Logger
	initErr       error
)

// Options for set up zero logger
type Options struct {
	// metadata
	Env           string
	Service       string
	Level         *zerolog.Level
	ApplyToGlobal bool

	// Console
	ConsoleEnable        bool
	ConsolePipe          bool
	ConsoleFieldsExclude []string

	// File (JSON)
	FileEnable     bool
	FilePath       string
	FileMaxSizeMB  int // default 50
	FileMaxBackups int // default 10
	FileMaxAgeDays int // default 14
	FileCompress   bool

	// Time and Duration
	TimeFieldFormat      string
	DurationFieldUnit    time.Duration
	DurationFieldInteger bool

	// properties
	EnableCaller  bool
	EnablePIDHost bool

	// Sampling
	EnableSampling    bool
	BasicSampleN      uint32
	BurstSampleBurst  uint32
	BurstSamplePeriod time.Duration
	LevelSampling     *LevelSamplingOptions

	// Hooks
	Hooks []zerolog.Hook
}

type LevelSamplingOptions struct {
	Trace, Debug, Info, Warning, Error, Fatal, Panic *SamplerSpec
}
type SamplerSpec struct {
	Burst      uint32
	Period     time.Duration
	NextEveryN uint32
	BasicN     uint32
}

// --------- Singleton API ---------

func Init(opts *Options) error {
	once.Do(func() {
		lConsole, lFile, err := buildAll(opts)
		if err != nil {
			initErr = err
			return
		}
		consoleLogger, fileLogger = lConsole, lFile
	})
	return initErr
}

// Get: logger default based on config
func Console() zerolog.Logger { _ = Init(nil); return consoleLogger }
func File() zerolog.Logger    { _ = Init(nil); return fileLogger }

func SetLevel(l zerolog.Level) {
	zerolog.SetGlobalLevel(l)
	consoleLogger = consoleLogger.Level(l)
	fileLogger = fileLogger.Level(l)
}

// --------- DI (غیر Singleton) ---------

// NewTargets: سه logger جدا (console/file/both) می‌دهد
func NewTargets(opts *Options) (console zerolog.Logger, file zerolog.Logger, err error) {
	c, f, err := buildAll(opts)
	return c, f, err
}

// --------- داخلی ---------

func buildAll(in *Options) (consoleOnly, fileOnly zerolog.Logger, err error) {
	o := populateOptions(in)

	// zerolog global configs
	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack
	if o.TimeFieldFormat != "" {
		zerolog.TimeFieldFormat = o.TimeFieldFormat
	} else {
		zerolog.TimeFieldFormat = time.RFC3339Nano
	}
	if o.DurationFieldUnit == 0 {
		o.DurationFieldUnit = time.Millisecond
	}
	zerolog.DurationFieldUnit = o.DurationFieldUnit
	zerolog.DurationFieldInteger = o.DurationFieldInteger

	// level
	level := resolveLevel(o)
	if o.ApplyToGlobal {
		zerolog.SetGlobalLevel(level)
	}

	// writers
	var wConsole io.Writer = io.Discard
	var wFile io.Writer = io.Discard

	if o.ConsoleEnable {
		pc, cerr := buildConsoleWriter(o)
		if cerr != nil {
			return consoleOnly, fileOnly, fmt.Errorf("console writer: %w", cerr)
		}
		wConsole = pc
	}
	if o.FileEnable {
		pf, ferr := buildFileWriter(o)
		if ferr != nil {
			return consoleOnly, fileOnly, fmt.Errorf("file writer: %w", ferr)
		}
		wFile = pf
	}

	// Build contextual base (shared)
	base := func(w io.Writer) zerolog.Logger {
		ctx := zerolog.New(zerolog.SyncWriter(w)).Level(level).With().Timestamp()

		hostname, _ := os.Hostname()
		pid := os.Getpid()
		buildInfo, _ := debug.ReadBuildInfo()
		var gitRevision, goVersion string
		if buildInfo != nil {
			goVersion = buildInfo.GoVersion
			for _, s := range buildInfo.Settings {
				if s.Key == "vcs.revision" {
					gitRevision = s.Value
					break
				}
			}
		} else {
			goVersion = strings.TrimSpace(os.Getenv("GO_VERSION"))
			gitRevision = strings.TrimSpace(os.Getenv("GIT_COMMIT"))
		}

		if o.Service != "" {
			ctx = ctx.Str("service", o.Service)
		}
		if o.Env != "" {
			ctx = ctx.Str("env", o.Env)
		}
		if o.EnablePIDHost {
			ctx = ctx.Int("pid", pid)
			if hostname != "" {
				ctx = ctx.Str("host", hostname)
			}
		}
		if gitRevision != "" {
			ctx = ctx.Str("git_revision", gitRevision)
		}
		if goVersion != "" {
			ctx = ctx.Str("go_version", goVersion)
		}
		l := ctx.Logger()
		if o.EnableCaller {
			l = l.With().Caller().Logger()
		}
		// sampling
		if o.LevelSampling != nil {
			l = l.Sample(buildLevelSampler(o.LevelSampling))
		} else if o.EnableSampling {
			l = l.Sample(buildSampler(o))
		}
		// hooks
		for _, h := range o.Hooks {
			l = l.Hook(h)
		}
		return l
	}

	// Assemble target loggers
	consoleOnly = base(wConsole)
	fileOnly = base(wFile)

	return
}

// Console writer (Pipe format) یا ConsoleWriter معمولی
func buildConsoleWriter(o *Options) (io.Writer, error) {
	if o.ConsolePipe {
		return &pipeConsoleWriter{
			Out: os.Stdout,
			// Out:          os.Stderr,
			TimeKey:      "time",
			LevelKey:     "level",
			MessageKey:   "message",
			TimeLayout:   time.RFC3339,
			UppercaseLvl: true,
			Exclude:      setFromSlice(o.ConsoleFieldsExclude),
		}, nil
	}
	// اگر Pipe نمی‌خوای، ConsoleWriter استاندارد با فرمت خود zerolog
	cw := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: time.RFC3339,
	}
	if len(o.ConsoleFieldsExclude) > 0 {
		cw.FieldsExclude = append([]string(nil), o.ConsoleFieldsExclude...)
	}
	return cw, nil
}

// File writer: JSON + lumberjack
func buildFileWriter(o *Options) (io.Writer, error) {
	if o.FilePath == "" {
		o.FilePath = "logs/app.log"
	}
	if dir := filepath.Dir(o.FilePath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}
	size := o.FileMaxSizeMB
	if size <= 0 {
		size = 50
	}
	backups := o.FileMaxBackups
	if backups <= 0 {
		backups = 10
	}
	age := o.FileMaxAgeDays
	if age < 0 {
		age = 0
	}
	lj := &lumberjack.Logger{
		Filename:   o.FilePath,
		MaxSize:    size,
		MaxBackups: backups,
		MaxAge:     age,
		Compress:   o.FileCompress,
	}
	return lj, nil
}

// Pipe-console writer: JSON event → "time | LEVEL | msg | k=v ..."
type pipeConsoleWriter struct {
	Out          io.Writer
	TimeKey      string
	LevelKey     string
	MessageKey   string
	TimeLayout   string
	UppercaseLvl bool
	Exclude      map[string]struct{}
}

func (w *pipeConsoleWriter) Write(p []byte) (int, error) {
	var m map[string]any
	if err := json.Unmarshal(p, &m); err != nil {
		// اگر JSON نبود، خام چاپ کن
		return w.Out.Write(p)
	}
	// time
	ts := ""
	if v, ok := m[w.TimeKey]; ok {
		switch t := v.(type) {
		case string:
			ts = t
		case float64:
			// اگر unix ms/seconds بود
			// اینجا ساده می‌گیریم: به رشته تبدیل کن
			ts = fmt.Sprintf("%.0f", t)
		default:
			ts = fmt.Sprint(v)
		}
		delete(m, w.TimeKey)
	}
	// level
	lvl := ""
	if v, ok := m[w.LevelKey]; ok {
		lvl = fmt.Sprint(v)
		if w.UppercaseLvl {
			lvl = strings.ToUpper(lvl)
		}
		delete(m, w.LevelKey)
	}
	// message
	msg := ""
	if v, ok := m[w.MessageKey]; ok {
		msg = fmt.Sprint(v)
		delete(m, w.MessageKey)
	}
	// باقی فیلدها → k=v
	parts := make([]string, 0, len(m)+3)
	if ts != "" {
		parts = append(parts, ts)
	}
	if lvl != "" {
		parts = append(parts, lvl)
	}
	if msg != "" {
		parts = append(parts, msg)
	}
	// مرتب‌سازی اختیاری: برای سادگی، فقط عبور خطی
	for k, v := range m {
		if _, skip := w.Exclude[k]; skip {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s=%v", k, v))
	}
	line := strings.Join(parts, " | ") + "\n"
	return w.Out.Write([]byte(line))
}

func setFromSlice(ss []string) map[string]struct{} {
	if len(ss) == 0 {
		return nil
	}
	out := make(map[string]struct{}, len(ss))
	for _, s := range ss {
		s = strings.TrimSpace(s)
		if s != "" {
			out[s] = struct{}{}
		}
	}
	return out
}

// --------- helpers---------

func buildSampler(o *Options) zerolog.Sampler {
	if o.BurstSampleBurst > 0 && o.BurstSamplePeriod > 0 {
		var next zerolog.Sampler
		if o.BasicSampleN > 0 {
			next = &zerolog.BasicSampler{N: o.BasicSampleN}
		}
		return &zerolog.BurstSampler{
			Burst:       o.BurstSampleBurst,
			Period:      o.BurstSamplePeriod,
			NextSampler: next,
		}
	}
	if o.BasicSampleN > 0 {
		return &zerolog.BasicSampler{N: o.BasicSampleN}
	}
	return allowAllSampler{}
}

func buildLevelSampler(ls *LevelSamplingOptions) zerolog.LevelSampler {
	var s zerolog.LevelSampler
	s.TraceSampler = samplerSpec(ls.Trace)
	s.DebugSampler = samplerSpec(ls.Debug)
	s.InfoSampler = samplerSpec(ls.Info)
	s.WarnSampler = samplerSpec(ls.Warning)
	s.ErrorSampler = samplerSpec(ls.Error)
	// s.FatalSampler = samplerSpec(ls.Fatal)
	// s.PanicSampler = samplerSpec(ls.Panic)
	return s
}

func samplerSpec(spec *SamplerSpec) zerolog.Sampler {
	if spec == nil {
		return nil
	}
	if spec.Burst > 0 && spec.Period > 0 {
		var next zerolog.Sampler
		if spec.NextEveryN > 0 {
			next = &zerolog.BasicSampler{N: spec.NextEveryN}
		} else if spec.BasicN > 0 {
			next = &zerolog.BasicSampler{N: spec.BasicN}
		}
		return &zerolog.BurstSampler{
			Burst:       spec.Burst,
			Period:      spec.Period,
			NextSampler: next,
		}
	}
	if spec.BasicN > 0 {
		return &zerolog.BasicSampler{N: spec.BasicN}
	}
	return nil
}

func resolveLevel(o *Options) zerolog.Level {
	if o.Level != nil {
		return clampLevel(*o.Level)
	}
	if s := strings.TrimSpace(os.Getenv("LOG_LEVEL")); s != "" {
		if lvl, ok := tryParseLevel(s); ok {
			return clampLevel(lvl)
		}
	}
	if strings.ToLower(o.Env) == "development" {
		return zerolog.DebugLevel
	}
	return zerolog.InfoLevel
}

func clampLevel(l zerolog.Level) zerolog.Level {
	if l < zerolog.TraceLevel {
		return zerolog.TraceLevel
	}
	if l > zerolog.PanicLevel {
		return zerolog.PanicLevel
	}
	return l
}

func tryParseLevel(s string) (zerolog.Level, bool) {
	if n, err := strconv.Atoi(s); err == nil {
		if n <= int(zerolog.TraceLevel) {
			return zerolog.TraceLevel, true
		}
		if n >= int(zerolog.PanicLevel) {
			return zerolog.PanicLevel, true
		}
		return zerolog.Level(n), true
	}
	switch strings.ToLower(s) {
	case "trace":
		return zerolog.TraceLevel, true
	case "debug":
		return zerolog.DebugLevel, true
	case "info":
		return zerolog.InfoLevel, true
	case "warn", "warning":
		return zerolog.WarnLevel, true
	case "error":
		return zerolog.ErrorLevel, true
	case "fatal":
		return zerolog.FatalLevel, true
	case "panic":
		return zerolog.PanicLevel, true
	default:
		return zerolog.InfoLevel, false
	}
}

func populateOptions(in *Options) *Options {
	o := &Options{}
	if in != nil {
		*o = *in
	}

	// default‌ها
	if o.Env == "" {
		o.Env = getenv("APP_ENV", "development")
	}
	if o.FilePath == "" {
		o.FilePath = getenv("LOG_FILE_PATH", "logs/app.log")
	}
	if !o.ConsoleEnable && !o.FileEnable {
		// اگر هیچ‌کدام تعیین نشد: dev → console، prod → file
		if strings.ToLower(o.Env) == "development" {
			o.ConsoleEnable = true
			o.ConsolePipe = true
		} else {
			o.FileEnable = true
		}
	}
	if o.FileEnable {
		if o.FileMaxSizeMB == 0 {
			o.FileMaxSizeMB = atoi(getenv("LOG_FILE_MAX_MB", "50"))
		}
		if o.FileMaxBackups == 0 {
			o.FileMaxBackups = atoi(getenv("LOG_FILE_BACKUPS", "10"))
		}
		if o.FileMaxAgeDays == 0 {
			o.FileMaxAgeDays = atoi(getenv("LOG_FILE_MAX_AGE_DAYS", "14"))
		}
		o.FileCompress = o.FileCompress || strings.EqualFold(getenv("LOG_FILE_COMPRESS", "true"), "true")
	}
	if o.TimeFieldFormat == "" {
		o.TimeFieldFormat = getenv("LOG_TIME_FORMAT", time.RFC3339Nano)
	}
	if o.DurationFieldUnit == 0 {
		switch strings.ToLower(getenv("LOG_DURATION_UNIT", "ms")) {
		case "ns":
			o.DurationFieldUnit = time.Nanosecond
		case "us", "µs":
			o.DurationFieldUnit = time.Microsecond
		case "ms":
			o.DurationFieldUnit = time.Millisecond
		case "s":
			o.DurationFieldUnit = time.Second
		default:
			o.DurationFieldUnit = time.Millisecond
		}
	}
	if !o.EnableCaller {
		o.EnableCaller = strings.EqualFold(getenv("LOG_CALLER", "true"), "true")
	}
	if !o.EnablePIDHost {
		o.EnablePIDHost = strings.EqualFold(getenv("LOG_PID_HOST", "true"), "true")
	}
	if !o.ApplyToGlobal {
		o.ApplyToGlobal = strings.EqualFold(getenv("LOG_APPLY_TO_GLOBAL", "true"), "true")
	}
	// ConsolePipe default در dev
	if !o.ConsolePipe && strings.ToLower(o.Env) == "development" {
		o.ConsolePipe = true
	}
	return o
}

func getenv(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}
func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

// allowAllSampler: همه‌ی لاگ‌ها رو اجازه میده (no-op)
type allowAllSampler struct{}

func (allowAllSampler) Sample(zerolog.Level) bool { return true }

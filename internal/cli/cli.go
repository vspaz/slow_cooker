package cli

import (
	"bufio"
	"flag"
	"fmt"
	"github.com/vspaz/slow_cooker/internal/http_client"
	"net/url"
	"os"
	"path"
	"strings"
	"time"
)

type Args struct {
	Qps              int
	Concurrency      int
	IterationCount   uint64
	Host             string
	Method           string
	Interval         time.Duration
	NoReuse          bool
	Compress         bool
	ClientTimeout    time.Duration
	NoLatencySummary bool
	ReportLatencyCsv string
	LatencyUnit      string
	LatencyDuration  time.Duration
	Help             bool
	TotalRequests    uint64
	Headers          map[string]string
	Data             string
	MetricAddr       string
	HashValue        uint64
	HashSampleRate   float64
	DstUrls          []*url.URL
}

func exUsage(msg string, args ...interface{}) {
	fmt.Fprintln(os.Stderr, fmt.Sprintf(msg, args...))
	fmt.Fprintln(os.Stderr, "Try --help for help.")
	os.Exit(64)
}

func loadURLs(urldest string) []*url.URL {
	var urls []*url.URL
	var err error
	var scanner *bufio.Scanner

	if strings.HasPrefix(urldest, "@") {
		var file *os.File
		filePath := urldest[1:]
		if filePath == "-" {
			file = os.Stdin
		} else {
			file, err = os.Open(filePath)
			if err != nil {
				fmt.Fprintf(os.Stderr, err.Error())
				os.Exit(1)
			}
			defer file.Close()
		}
		scanner = bufio.NewScanner(file)
	} else {
		scanner = bufio.NewScanner(strings.NewReader(urldest))
	}

	for i := 1; scanner.Scan(); i++ {
		line := scanner.Text()
		URL, err := url.Parse(line)
		if err != nil {
			exUsage("invalid URL on line %d: '%s': %s\n", i, line, err.Error())
		} else if URL.Scheme == "" {
			exUsage("invalid URL on line %d: '%s': Missing scheme\n", i, line)
		} else if URL.Host == "" {
			exUsage("invalid URL on line %d: '%s': Missing host\n", i, line)
		}
		urls = append(urls, URL)
	}

	return urls
}

func GetArgs() Args {
	qps := flag.Int("qps", 1, "QPS to send to backends per request thread")
	concurrency := flag.Int("concurrency", 1, "Number of request threads")
	iterationCount := flag.Uint64("iterations", 0, "Number of iterations (0 for infinite)")
	host := flag.String("host", "", "value of Host header to set")
	method := flag.String("method", "POST", "HTTP method to use")
	interval := flag.Duration("interval", 10*time.Second, "reporting interval")
	noreuse := flag.Bool("noreuse", false, "don't reuse connections")
	compress := flag.Bool("compress", false, "use compression")
	clientTimeout := flag.Duration("timeout", 10*time.Second, "individual request timeout")
	noLatencySummary := flag.Bool("noLatencySummary", false, "suppress the final latency summary")
	reportLatenciesCSV := flag.String("reportLatenciesCSV", "",
		"filename to output hdrhistogram latencies in CSV")
	latencyUnit := flag.String("latencyUnit", "ms", "latency units [ms|us|ns]")
	help := flag.Bool("help", false, "show help message")
	totalRequests := flag.Uint64("totalRequests", 0, "total number of requests to send before exiting")
	headerString := flag.String("headers", "", "HTTP request headers separated by a comma, e.g. \"Content-Type: application/json\"")
	data := flag.String("data", "", "HTTP request data")
	metricAddr := flag.String("metric-addr", "", "address to serve metrics on")
	hashValue := flag.Uint64("hashValue", 0, "fnv-1a hash value to check the request body against")
	hashSampleRate := flag.Float64("hashSampleRate", 0.0, "Sampe Rate for checking request body's hash. Interval in the range of [0.0, 1.0]")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s <url> [flags]\n", path.Base(os.Args[0]))
		flag.PrintDefaults()
	}

	flag.Parse()

	if *help {
		flag.Usage()
		os.Exit(64)
	}

	if flag.NArg() != 1 {
		exUsage("Expecting one argument: the target url to test, e.g. http://localhost:4140/")
	}

	if *qps < 1 {
		exUsage("qps must be at least 1")
	}

	if *concurrency < 1 {
		exUsage("concurrency must be at least 1")
	}

	latencyDur := time.Millisecond
	if *latencyUnit == "ms" {
		latencyDur = time.Millisecond
	} else if *latencyUnit == "us" {
		latencyDur = time.Microsecond
	} else if *latencyUnit == "ns" {
		latencyDur = time.Nanosecond
	} else {
		exUsage("latency unit should be [ms | us | ns].")
	}

	return Args{
		Qps:              *qps,
		Concurrency:      *concurrency,
		IterationCount:   *iterationCount,
		Host:             *host,
		Method:           *method,
		Interval:         *interval,
		NoReuse:          *noreuse,
		Compress:         *compress,
		ClientTimeout:    *clientTimeout,
		NoLatencySummary: *noLatencySummary,
		ReportLatencyCsv: *reportLatenciesCSV,
		LatencyUnit:      *latencyUnit,
		LatencyDuration:  latencyDur,
		Help:             *help,
		TotalRequests:    *totalRequests,
		Headers:          http_client.GetHeaders(*headerString),
		Data:             *data,
		MetricAddr:       *metricAddr,
		HashValue:        *hashValue,
		HashSampleRate:   *hashSampleRate,
		DstUrls:          loadURLs(flag.Arg(0)),
	}
}

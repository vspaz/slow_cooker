package cmd

import (
	"fmt"
	"github.com/vspaz/slow_cooker/internal/cli"
	"github.com/vspaz/slow_cooker/internal/hdrreport"
	"github.com/vspaz/slow_cooker/internal/http_client"
	"github.com/vspaz/slow_cooker/internal/ring"
	"github.com/vspaz/slow_cooker/internal/window"
	"hash/fnv"
	"io"
	"log"
	"math"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/HdrHistogram/hdrhistogram-go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// CalcTimeToWait calculates how many Nanoseconds to wait between actions.
func CalcTimeToWait(qps *int) time.Duration {
	return time.Duration(int(time.Second) / *qps)
}

var reqID = uint64(0)

var shouldFinish = false
var shouldFinishLock sync.RWMutex

// finishSendingTraffic signals the system to stop sending traffic and clean up after itself.
func finishSendingTraffic() {
	shouldFinishLock.Lock()
	shouldFinish = true
	shouldFinishLock.Unlock()
}

func loadData(data string) []byte {
	var file *os.File
	var requestData []byte
	var err error
	if strings.HasPrefix(data, "@") {
		filePath := data[1:]
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

		requestData, err = io.ReadAll(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, err.Error())
			os.Exit(1)
		}
	} else {
		requestData = []byte(data)
	}

	return requestData
}

var (
	promRequests = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "requests",
		Help: "Number of requests",
	})

	promSuccesses = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "successes",
		Help: "Number of successful requests",
	})

	promLatencyMSHistogram = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "latency_ms",
		Help: "RPC latency distributions in milliseconds.",
		// 50 exponential buckets ranging from 0.5 ms to 3 minutes
		// TODO: make this tunable
		Buckets: prometheus.ExponentialBuckets(0.5, 1.3, 50),
	})
	promLatencyUSHistogram = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "latency_us",
		Help: "RPC latency distributions in microseconds.",
		// 50 exponential buckets ranging from 1 us to 2.4 seconds
		// TODO: make this tunable
		Buckets: prometheus.ExponentialBuckets(1, 1.35, 50),
	})
	promLatencyNSHistogram = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "latency_ns",
		Help: "RPC latency distributions in nanoseconds.",
		// 50 exponential buckets ranging from 1 ns to 0.4 seconds
		// TODO: make this tunable
		Buckets: prometheus.ExponentialBuckets(1, 1.5, 50),
	})
)

func registerMetrics() {
	prometheus.MustRegister(promRequests)
	prometheus.MustRegister(promSuccesses)
	prometheus.MustRegister(promLatencyMSHistogram)
	prometheus.MustRegister(promLatencyUSHistogram)
	prometheus.MustRegister(promLatencyNSHistogram)
}

// Sample Rate is between [0.0, 1.0] and determines what percentage of request bodies
// should be checked that their hash matches a known hash.
func shouldCheckHash(sampleRate float64) bool {
	return rand.Float64() < sampleRate
}

func Run() {
	args := cli.GetArgs()

	latencyDurNS := args.LatencyDuration.Nanoseconds()
	msInNS := time.Millisecond.Nanoseconds()
	usInNS := time.Microsecond.Nanoseconds()

	hosts := strings.Split(args.Host, ",")

	requestData := loadData(args.Data)

	iteration := uint64(0)

	// Response tracking metadata.
	count := uint64(0)
	size := uint64(0)
	good := uint64(0)
	bad := uint64(0)
	failed := uint64(0)
	minValue := int64(math.MaxInt64)
	maxValue := int64(0)
	failedHashCheck := int64(0)

	// dayInTimeUnits represents the number of time units (ms, us, or ns) in a 24-hour day.
	dayInTimeUnits := int64(24 * time.Hour / args.LatencyDuration)

	hist := hdrhistogram.New(0, dayInTimeUnits, 3)
	globalHist := hdrhistogram.New(0, dayInTimeUnits, 3)
	latencyHistory := ring.New(5)
	received := make(chan *http_client.MeasuredResponse)
	timeout := time.After(args.Interval)
	timeToWait := CalcTimeToWait(&args.Qps)
	var totalTrafficTarget int
	totalTrafficTarget = args.Qps * args.Concurrency * int(args.Interval.Seconds())

	client := http_client.NewClient(args.Compress, args.NoReuse, args.Concurrency, args.ClientTimeout)
	var sendTraffic sync.WaitGroup
	// The time portion of the header can change due to timezone.
	timeLen := len(time.Now().Format(time.RFC3339))
	timePadding := strings.Repeat(" ", timeLen-len("# "))
	intLen := len(fmt.Sprintf("%s", args.Interval))
	intPadding := strings.Repeat(" ", intLen-2)

	if len(args.DstUrls) == 1 {
		fmt.Printf("# sending %d %s req/s with concurrency=%d to %s ...\n", (args.Qps * args.Concurrency), args.Method, args.Concurrency, args.DstUrls[0])
	} else {
		fmt.Printf("# sending %d %s req/s with concurrency=%d using url list %s ...\n", (args.Qps * args.Concurrency), args.Method, args.Concurrency, args.DstUrls[1:])
	}

	fmt.Printf("# %s iter   good/b/f t   goal%% %s minValue [p50 p95 p99  p999]  maxValue bhash change\n", timePadding, intPadding)
	stride := args.Concurrency
	if stride > len(args.DstUrls) {
		stride = 1
	}
	for i := 0; i < args.Concurrency; i++ {
		ticker := time.NewTicker(timeToWait)
		go func(offset int) {
			y := offset
			// For each goroutine we want to reuse a buffer for performance reasons.
			bodyBuffer := make([]byte, 50000)
			sendTraffic.Add(1)
			for range ticker.C {
				var checkHash bool
				hasher := fnv.New64a()
				if args.HashSampleRate > 0.0 {
					checkHash = shouldCheckHash(args.HashSampleRate)
				} else {
					checkHash = false
				}
				shouldFinishLock.RLock()
				if !shouldFinish {
					shouldFinishLock.RUnlock()
					http_client.SendRequest(client, args.Method, args.DstUrls[y], hosts[rand.Intn(len(hosts))], args.Headers, requestData, atomic.AddUint64(&reqID, 1), args.NoReuse, args.HashValue, checkHash, hasher, received, bodyBuffer)
				} else {
					shouldFinishLock.RUnlock()
					sendTraffic.Done()
					return
				}
				y += stride
				if y >= len(args.DstUrls) {
					y = offset
				}
			}
		}(i % len(args.DstUrls))
	}

	cleanup := make(chan bool, 3)
	interrupted := make(chan os.Signal, 2)
	signal.Notify(interrupted, syscall.SIGINT)

	if args.MetricAddr != "" {
		registerMetrics()
		go func() {
			http.Handle("/metrics", promhttp.Handler())
			http.ListenAndServe(args.MetricAddr, nil)
		}()
	}

	for {
		select {
		// If we get a SIGINT, then start the shutdown process.
		case <-interrupted:
			cleanup <- true
		case <-cleanup:
			finishSendingTraffic()
			if !args.NoLatencySummary {
				hdrreport.PrintLatencySummary(globalHist)
			}
			if args.ReportLatencyCsv != "" {
				err := hdrreport.WriteReportCSV(&args.ReportLatencyCsv, globalHist)
				if err != nil {
					log.Panicf("Unable to write Latency CSV file: %v\n", err)
				}
			}
			go func() {
				// Don't Wait() in the event loop or else we'll block the workers
				// from draining.
				sendTraffic.Wait()
				os.Exit(0)
			}()
		case t := <-timeout:
			// When all requests are failures, ensure we don't accidentally
			// print out a monstrously huge number.
			if minValue == math.MaxInt64 {
				minValue = 0
			}
			// Periodically print stats about the request load.
			percentAchieved := int(math.Min((((float64(good) + float64(bad)) /
				float64(totalTrafficTarget)) * 100), 100))

			lastP99 := int(hist.ValueAtQuantile(99))
			// We want the change indicator to be based on
			// how far away the current value is from what
			// we've seen historically. This is why we call
			// CalculateChangeIndicator() first and then Push()
			changeIndicator := window.CalculateChangeIndicator(latencyHistory.Items, lastP99)
			latencyHistory.Push(lastP99)

			fmt.Printf("%s %4d %6d/%1d/%1d %d %3d%% %s %3d [%3d %3d %3d %4d ] %4d %6d %s\n",
				t.Format(time.RFC3339),
				iteration,
				good,
				bad,
				failed,
				totalTrafficTarget,
				percentAchieved,
				args.Interval,
				minValue,
				hist.ValueAtQuantile(50),
				hist.ValueAtQuantile(95),
				hist.ValueAtQuantile(99),
				hist.ValueAtQuantile(999),
				maxValue,
				failedHashCheck,
				changeIndicator)

			iteration++

			if args.IterationCount > 0 && iteration >= args.IterationCount {
				cleanup <- true
			}
			count = 0
			size = 0
			good = 0
			bad = 0
			minValue = math.MaxInt64
			maxValue = 0
			failed = 0
			failedHashCheck = 0
			hist.Reset()
			timeout = time.After(args.Interval)

			if args.TotalRequests != 0 && reqID > args.TotalRequests {
				cleanup <- true
			}
		case managedResp := <-received:
			count++
			promRequests.Inc()
			if managedResp.Err != nil {
				fmt.Fprintln(os.Stderr, managedResp.Err)
				failed++
			} else {
				respLatencyNS := managedResp.Latency.Nanoseconds()
				latency := respLatencyNS / latencyDurNS

				size += managedResp.Sz
				if managedResp.FailedHashCheck {
					failedHashCheck++
				}
				if managedResp.Code >= 200 && managedResp.Code < 500 {
					good++
					promSuccesses.Inc()
					promLatencyMSHistogram.Observe(float64(respLatencyNS / msInNS))
					promLatencyUSHistogram.Observe(float64(respLatencyNS / usInNS))
					promLatencyNSHistogram.Observe(float64(respLatencyNS))
				} else {
					bad++
				}

				if latency < minValue {
					minValue = latency
				}

				if latency > maxValue {
					maxValue = latency
				}

				hist.RecordValue(latency)
				globalHist.RecordValue(latency)
			}
		}
	}
}

package generator

import (
	"fmt"
	"github.com/vspaz/slow_cooker/internal/cli"
	"github.com/vspaz/slow_cooker/internal/hdrreport"
	"github.com/vspaz/slow_cooker/internal/metrics"
	"github.com/vspaz/slow_cooker/internal/ring"
	"github.com/vspaz/slow_cooker/internal/window"
	"hash/fnv"
	"log"
	"math"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/HdrHistogram/hdrhistogram-go"
)

func Run() {
	args := cli.GetArgs()

	latencyDurNS := args.LatencyDuration.Nanoseconds()

	iteration := uint64(0)
	reqID := uint64(0)
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
	received := make(chan *MeasuredResponse)
	timeout := time.After(args.Interval)
	timeToWait := CalcTimeToWait(&args.Qps)
	totalTrafficTarget := args.Qps * args.Concurrency * int(args.Interval.Seconds())

	requestGenerator := NewRequestGenerator(&args)
	var sendTraffic sync.WaitGroup
	// The time portion of the header can change due to timezone.
	timeLen := len(time.Now().Format(time.RFC3339))
	timePadding := strings.Repeat(" ", timeLen-len("# "))
	intLen := len(fmt.Sprintf("%s", args.Interval))
	intPadding := strings.Repeat(" ", intLen-2)

	println(GetRequestInfo(&args))
	fmt.Printf("# %s iter   good/b/f t   goal%% %s minValue [p50 p95 p99  p999]  maxValue bhash change\n", timePadding, intPadding)
	stride := args.Concurrency
	if stride > len(args.DstUrls) {
		stride = 1
	}
	isFinish := atomic.Bool{}
	for i := 0; i < args.Concurrency; i++ {
		ticker := time.NewTicker(timeToWait)
		go func(offset int) {
			initialOffset := offset
			// For each goroutine we want to reuse a buffer for performance reasons.
			bodyBuffer := make([]byte, 50000)
			sendTraffic.Add(1)
			for range ticker.C {
				checkHash := false
				hasher := fnv.New64a()
				if args.HashSampleRate > 0.0 {
					checkHash = ShouldCheckHash(args.HashSampleRate)
				}

				if isFinish.Load() {
					sendTraffic.Done()
					return
				}

				requestGenerator.DoRequest(
					initialOffset,
					atomic.AddUint64(&reqID, 1),
					checkHash,
					hasher,
					received,
					bodyBuffer,
				)

				initialOffset += stride
				if initialOffset >= len(args.DstUrls) {
					initialOffset = offset
				}
			}
		}(i % len(args.DstUrls))
	}

	cleanup := make(chan bool, 3)
	interrupted := make(chan os.Signal, 2)
	signal.Notify(interrupted, syscall.SIGINT)

	if args.MetricAddr != "" {
		metrics.RegisterMetrics()
		go metrics.RunServer(&args)
	}

	for {
		select {
		// If we get a SIGINT, then start the shutdown process.
		case <-interrupted:
			cleanup <- true
		case <-cleanup:
			isFinish.Store(true)
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
			metrics.PromRequests.Inc()
			if managedResp.Err != nil {
				fmt.Fprintln(os.Stderr, managedResp.Err)
				failed++
			} else {
				respLatencyNS := managedResp.Latency.Nanoseconds()

				size += managedResp.Sz
				if managedResp.FailedHashCheck {
					failedHashCheck++
				}
				if managedResp.Code/100 == 2 {
					good++
					metrics.UpdateLatencyMetrics(respLatencyNS)
				} else {
					bad++
				}

				latency := respLatencyNS / latencyDurNS

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

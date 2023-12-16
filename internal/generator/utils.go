package generator

import (
	"fmt"
	"github.com/vspaz/slow_cooker/internal/cli"
	"math/rand"
	"time"
)

// CalcTimeToWait calculates how many Nanoseconds to wait between actions.
func CalcTimeToWait(qps *int) time.Duration {
	return time.Duration(int(time.Second) / *qps)
}

// Sample Rate is between [0.0, 1.0] and determines what percentage of request bodies
// should be checked that their hash matches a known hash.
func ShouldCheckHash(sampleRate float64) bool {
	return rand.Float64() < sampleRate
}

func GetRequestInfo(args *cli.Args) string {
	if len(args.DstUrls) == 1 {
		return fmt.Sprintf(
			"# sending %d %s req/s with concurrency=%d to %s ...\n",
			(args.Qps * args.Concurrency), args.Method, args.Concurrency, args.DstUrls[0])
	}
	return fmt.Sprintf(
		"# sending %d %s req/s with concurrency=%d using url list %s ...\n",
		(args.Qps * args.Concurrency), args.Method, args.Concurrency, args.DstUrls[1:])
}

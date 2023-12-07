package utils

import (
	"fmt"
	"github.com/vspaz/slow_cooker/internal/cli"
	"io"
	"math/rand"
	"os"
	"strings"
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

func LoadData(data string) []byte {
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

func GetRequestInfo(args *cli.Args) string {
	if len(args.DstUrls) == 1 {
		return fmt.Sprintf("# sending %d %s req/s with concurrency=%d to %s ...\n", (args.Qps * args.Concurrency), args.Method, args.Concurrency, args.DstUrls[0])
	}
	return fmt.Sprintf("# sending %d %s req/s with concurrency=%d using url list %s ...\n", (args.Qps * args.Concurrency), args.Method, args.Concurrency, args.DstUrls[1:])
}

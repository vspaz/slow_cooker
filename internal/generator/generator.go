package generator

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"github.com/vspaz/slow_cooker/internal/cli"
	"hash"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"os"
	"strconv"
	"time"
)

type RequestGenerator struct {
	httpClient *http.Client
	NoReuse    bool
	HashValue  uint64
}

func NewRequestGenerator(args *cli.Args) *RequestGenerator {
	tr := http.Transport{
		DisableCompression:  !args.Compress,
		DisableKeepAlives:   args.NoReuse,
		MaxIdleConnsPerHost: args.Concurrency,
		Proxy:               http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout: 5 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout: 5 * time.Second,
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
	}
	return &RequestGenerator{
		httpClient: &http.Client{
			Timeout:   args.ClientTimeout,
			Transport: &tr,
		},
		NoReuse:   args.NoReuse,
		HashValue: args.HashValue,
	}
}

// MeasuredResponse holds metadata about the response
// we receive from the server under test.
type MeasuredResponse struct {
	Sz              uint64
	Code            int
	Latency         time.Duration
	Timeout         bool
	FailedHashCheck bool
	Err             error
}

func (c *RequestGenerator) DoRequest(
	method string,
	url string,
	host string,
	headers map[string]string,
	requestData []byte,
	reqID uint64,
	checkHash bool,
	hasher hash.Hash64,
	received chan *MeasuredResponse,
	bodyBuffer []byte,
) {
	req, err := http.NewRequest(method, url, bytes.NewBuffer(requestData))
	req.Close = c.NoReuse
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		fmt.Fprintf(os.Stderr, "\n")
	}
	if host != "" {
		req.Host = host
	}
	req.Header.Add("Sc-Req-Id", strconv.FormatUint(reqID, 10))
	for k, v := range headers {
		req.Header.Add(k, v)
	}

	var elapsed time.Duration
	start := time.Now()

	trace := &httptrace.ClientTrace{
		GotFirstResponseByte: func() {
			elapsed = time.Since(start)
		},
	}

	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))
	response, err := c.httpClient.Do(req)

	if err != nil {
		received <- &MeasuredResponse{Err: err}
	} else {
		defer response.Body.Close()
		if !checkHash {
			if sz, err := io.CopyBuffer(io.Discard, response.Body, bodyBuffer); err == nil {

				received <- &MeasuredResponse{
					Sz:      uint64(sz),
					Code:    response.StatusCode,
					Latency: elapsed}
			} else {
				received <- &MeasuredResponse{Err: err}
			}
		} else {
			if byteArray, err := io.ReadAll(response.Body); err != nil {
				received <- &MeasuredResponse{Err: err}
			} else {
				hasher.Write(byteArray)
				sum := hasher.Sum64()
				failedHashCheck := false
				if c.HashValue != sum {
					failedHashCheck = true
				}
				received <- &MeasuredResponse{
					Sz:              uint64(len(byteArray)),
					Code:            response.StatusCode,
					Latency:         elapsed,
					FailedHashCheck: failedHashCheck}
			}
		}
	}
}

package http_client

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"hash"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

func GetHeaders(text string) map[string]string {
	headerNameToValue := make(map[string]string)
	headers := strings.Split(text, ",")
	for _, header := range headers {
		headerNameAndValue := strings.Split(header, ":")
		if len(headerNameAndValue) == 2 {
			headerName := strings.TrimSpace(headerNameAndValue[0])
			headerValue := strings.TrimSpace(headerNameAndValue[1])
			if len(headerName) > 0 {
				headerNameToValue[headerName] = headerValue
			}
		}
	}
	return headerNameToValue
}

func NewClient(
	compress bool,
	noreuse bool,
	maxConn int,
	timeout time.Duration,
) *http.Client {
	tr := http.Transport{
		DisableCompression:  !compress,
		DisableKeepAlives:   noreuse,
		MaxIdleConnsPerHost: maxConn,
		Proxy:               http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout: 5 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout: 5 * time.Second,
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: &tr,
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

func SendRequest(
	client *http.Client,
	method string,
	url *url.URL,
	host string,
	headers map[string]string,
	requestData []byte,
	reqID uint64,
	noreuse bool,
	hashValue uint64,
	checkHash bool,
	hasher hash.Hash64,
	received chan *MeasuredResponse,
	bodyBuffer []byte,
) {
	req, err := http.NewRequest(method, url.String(), bytes.NewBuffer(requestData))
	req.Close = noreuse
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
	response, err := client.Do(req)

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
				if hashValue != sum {
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

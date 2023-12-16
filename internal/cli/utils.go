package cli

import (
	"bufio"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
)

func exUsage(msg string, args ...interface{}) {
	fmt.Fprintln(os.Stderr, fmt.Sprintf(msg, args...))
	fmt.Fprintln(os.Stderr, "Try --help for help.")
	os.Exit(64)
}

func getHeaders(text string) map[string]string {
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

func loadBodyPayload(data string) []byte {
	var file *os.File
	var body []byte
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

		body, err = io.ReadAll(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, err.Error())
			os.Exit(1)
		}
	} else {
		body = []byte(data)
	}

	return body
}

func loadURLs(urldest string) []string {
	var urls []string
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
		urls = append(urls, URL.String())
	}

	return urls
}

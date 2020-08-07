package main

import (
	"bufio"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	_ "github.com/lib/pq"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type Target struct {
	Domain string `json:"domain"`
	Ip string `json:"ip"`
	Port string `json:"port"`
	IpStatus int `json:"ip_status_code"`
	IpLength int `json:"ip_response_len"`
	HostStatus int `json:"host_status_code"`
	HostLength int `json:"host_response_len"`
	LocalStatus int `json:"local_status_code"`
	LocalLength int `json:"local_response_len"`
	DomainAccessible bool `json:"domain_accessible"`
}

func NewTarget(targetParts string) Target {
	targetPieces := strings.Split(targetParts,",")
	return Target{strings.ReplaceAll(targetPieces[0],"\"",""),
		strings.ReplaceAll(targetPieces[1],"\"",""),
		strings.ReplaceAll(targetPieces[2],"\"",""),0,0,0,0,0,0,true}
}

func (t *Target) fetch() error {
	if t.Domain[0] == '*' {
		t.Domain = t.Domain[2:]
	}
	targetUrl := "https://"+t.Ip+":"+t.Port
	resp, err := client.Get(targetUrl)
	if err != nil {
		return err
	} else {
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		t.IpLength = len(bodyBytes)
		t.IpStatus = resp.StatusCode
		if debug {
			fmt.Println(targetUrl + " IP - L:" + strconv.Itoa(t.IpLength)+ " S:"+strconv.Itoa(t.IpStatus))
		}
	}
	resp.Body.Close()

	req, _ := http.NewRequest("GET", targetUrl, nil)
	req.Host = t.Domain
	resp, err = client.Do(req)
	if err != nil {
		return err
	} else {
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		t.HostLength = len(bodyBytes)
		t.HostStatus = resp.StatusCode
		if debug {
			fmt.Println(targetUrl + " Domain - L:" + strconv.Itoa(t.HostLength)+ " S:"+strconv.Itoa(t.HostStatus))
		}
	}
	resp.Body.Close()

	req.Host = "localhost"
	resp, err = client.Do(req)
	if err != nil {
		return err
	} else {
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		t.LocalLength = len(bodyBytes)
		t.LocalStatus = resp.StatusCode
		if debug {
			fmt.Println(targetUrl + " Local - L:" + strconv.Itoa(t.LocalLength)+ " S:"+strconv.Itoa(t.LocalStatus))
		}
	}
	resp.Body.Close()

	req, _ = http.NewRequest("GET", "https://"+t.Domain+":"+t.Port, nil)
	resp, err = client.Do(req)

	if err != nil {
		switch ty := err.(type) {
		case *net.OpError:
			if ty.Op == "dial" {
				println("Unknown host")
				t.DomainAccessible = false
			}
		case syscall.Errno:
			if ty == syscall.ECONNREFUSED {
				println("Connection refused")
				t.DomainAccessible = false
			}
		}
		if strings.Contains(err.Error(), "no such host") {
			t.DomainAccessible = false
		}
		return err
	}

	return nil
}

var client http.Client
var debug bool


func main() {
	targetPtr := flag.String("target", "", "Target file with hosts to fingerprint")
	timeoutPtr := flag.Int("timeout", 10, "timeout for connecting to servers")
	workersPtr := flag.Int("workers", 1024, "Number of workers to handle subnet jobs")
	debugPtr := flag.Bool("debug",false,"Enable to see any errors with fetching targets")
	flag.Parse()
	debug = *debugPtr

	dialer := net.Dialer{
		Timeout: time.Duration(*timeoutPtr) * time.Second,
		KeepAlive:time.Duration(*timeoutPtr) * time.Second,
	}

	defaultTransport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify:true,
			CipherSuites:nil,
			MaxVersion:tls.VersionTLS13,
		},
		DialContext: dialer.DialContext,
		MaxIdleConns: 100000,
		MaxIdleConnsPerHost: 4,
		IdleConnTimeout:time.Duration(*timeoutPtr) * time.Second,
		ResponseHeaderTimeout:time.Duration(*timeoutPtr) * time.Second,

	}
	client = http.Client{
		Transport: defaultTransport,
		Timeout:   time.Duration(*timeoutPtr) * time.Second,
	}
	wg := sync.WaitGroup{}
	concurrentGoroutines := make(chan struct{}, *workersPtr)

	inputFile, err := os.Open(*targetPtr)
	if err != nil {
		fmt.Println(err)
	}
	defer inputFile.Close()
	scanner := bufio.NewScanner(inputFile)
	for scanner.Scan() {
		go func(targ Target) {
			concurrentGoroutines <- struct{}{}
			wg.Add(1)
			fmt.Println("Job starting")

			err := targ.fetch()
			if err != nil &&  debug {
				fmt.Println(err)
			}
			jsonTarget, _ := json.Marshal(&targ)
			fmt.Println(string(jsonTarget))
			wg.Done()
			<-concurrentGoroutines
			fmt.Println("Job Done")
		}(NewTarget(scanner.Text()))
	}
	fmt.Println("Done ingesting targets")
	wg.Wait()
	fmt.Println("Done scanning")
}

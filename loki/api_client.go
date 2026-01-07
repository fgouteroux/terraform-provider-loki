package loki

// Largely copied from https://github.com/Mastercard/terraform-provider-restapi/blob/master/restapi/api_client.go

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/config"
)

type awsSigV4Config struct {
	region  string
	service string
}

type apiClientOpt struct {
	uri      string
	cert     string
	key      string
	ca       string
	token    string
	insecure bool
	username string
	password string
	proxyURL string
	headers  map[string]string
	timeout  int
	debug    bool
	awsSigV4 *awsSigV4Config
}

type apiClient struct {
	httpClient *http.Client
	uri        string
	insecure   bool
	token      string
	username   string
	password   string
	headers    map[string]string
	debug      bool
	awsSigV4   *awsSigV4Config
	awsCreds   aws.Credentials
}

// Make a new api client for RESTful calls
func NewAPIClient(opt *apiClientOpt) (*apiClient, error) {
	/* Remove any trailing slashes since we will append
	   to this URL with our own root-prefixed location */
	opt.uri = strings.TrimSuffix(opt.uri, "/")

	// Setup HTTPS client
	tlsConfig := &tls.Config{}

	// Set insecure verify
	if opt.insecure {
		tlsConfig.InsecureSkipVerify = true
	}

	if opt.cert != "" && opt.key != "" {
		var cert tls.Certificate
		var err error
		if strings.HasPrefix(opt.cert, "-----BEGIN") && strings.HasPrefix(opt.key, "-----BEGIN") {
			cert, err = tls.X509KeyPair([]byte(opt.cert), []byte(opt.key))
		} else {
			cert, err = tls.LoadX509KeyPair(opt.cert, opt.key)
		}
		if err != nil {
			return nil, err
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	if opt.ca != "" {
		var caCert []byte
		var err error
		if strings.HasPrefix(opt.ca, "-----BEGIN") {
			caCert = []byte(opt.ca)
		} else {
			caCert, err = os.ReadFile(opt.ca)

			if err != nil {
				return nil, err
			}
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)
		tlsConfig.RootCAs = caCertPool
	}

	tr := &http.Transport{
		TLSClientConfig: tlsConfig,
		Proxy:           http.ProxyFromEnvironment,
	}

	if opt.proxyURL != "" {
		log.Printf("api_client.go: Using proxy: %s\n", opt.proxyURL)
		proxy, err := url.Parse(opt.proxyURL)
		if err != nil {
			return nil, fmt.Errorf("error parsing proxy url: %s", err)
		}
		tr.Proxy = http.ProxyURL(proxy)
	}

	client := apiClient{
		httpClient: &http.Client{
			Timeout:   time.Second * time.Duration(opt.timeout),
			Transport: tr,
		},
		uri:      opt.uri,
		insecure: opt.insecure,
		token:    opt.token,
		username: opt.username,
		password: opt.password,
		headers:  opt.headers,
		debug:    opt.debug,
	}

	// Initialize AWS SigV4 if configured
	if opt.awsSigV4 != nil {
		client.awsSigV4 = opt.awsSigV4

		// Load AWS configuration from default credential chain
		cfg, err := config.LoadDefaultConfig(context.Background(),
			config.WithRegion(opt.awsSigV4.region),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to load AWS config: %w", err)
		}

		creds, err := cfg.Credentials.Retrieve(context.Background())
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve AWS credentials: %w", err)
		}

		client.awsCreds = creds

		if opt.debug {
			log.Printf("api_client.go: AWS SigV4 enabled for region=%s, service=%s\n",
				opt.awsSigV4.region, opt.awsSigV4.service)
		}
	}

	return &client, nil
}

/*
Helper function that handles sending/receiving and handling

	of HTTP data in and out.
*/
func (client *apiClient) sendRequest(method string, path, data string, headers map[string]string) (string, error) {
	fullURI := client.uri + path

	var req *http.Request
	var err error

	bodyBytes := []byte(data)
	buffer := bytes.NewBuffer(bodyBytes)

	if data == "" {
		req, err = http.NewRequest(method, fullURI, nil)
	} else {
		req, err = http.NewRequest(method, fullURI, buffer)
	}

	if err != nil {
		log.Fatal(err)
	}

	// AWS SigV4 signing (must be done before adding other headers)
	if client.awsSigV4 != nil {
		if err := client.signWithSigV4(req, bodyBytes); err != nil {
			return "", fmt.Errorf("AWS SigV4 signing failed: %w", err)
		}
	}

	if client.token != "" {
		req.Header.Set("Authorization", "Bearer "+client.token)
	}

	// Set client headers from provider
	if len(client.headers) > 0 {
		for n, v := range client.headers {
			req.Header.Set(n, v)
		}
	}

	// Set client headers from resource
	if len(headers) > 0 {
		for n, v := range headers {
			req.Header.Set(n, v)
		}
	}

	if client.username != "" && client.password != "" {
		/* ... and fall back to basic auth if configured */
		req.SetBasicAuth(client.username, client.password)
	}

	if client.debug {
		reqDump, err := httputil.DumpRequestOut(req, true)
		if err != nil {
			log.Fatal(err)
		}

		log.Printf("REQUEST:\n%s", string(reqDump))
	}

	resp, err := client.httpClient.Do(req)

	if err != nil {
		if client.debug {
			log.Printf("api_client.go: Error detected: %s\n", err)
		}
		return "", err
	}

	if client.debug {
		respDump, err := httputil.DumpResponse(resp, true)
		if err != nil {
			log.Fatal(err)
		}

		log.Printf("RESPONSE:\n%s", string(respDump))
	}

	bodyBytes, err2 := io.ReadAll(resp.Body)
	resp.Body.Close()

	if err2 != nil {
		return "", err2
	}
	body := string(bodyBytes)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return body, fmt.Errorf("unexpected response code '%d': %s", resp.StatusCode, body)
	}

	return body, nil
}

// signWithSigV4 signs the HTTP request using AWS Signature Version 4
func (client *apiClient) signWithSigV4(req *http.Request, body []byte) error {
	signer := v4.NewSigner()

	// Calculate payload hash
	hash := sha256.Sum256(body)
	payloadHash := hex.EncodeToString(hash[:])

	return signer.SignHTTP(
		context.Background(),
		client.awsCreds,
		req,
		payloadHash,
		client.awsSigV4.service,
		client.awsSigV4.region,
		time.Now(),
	)
}

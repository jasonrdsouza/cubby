package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

type CubbyClient struct {
	serverAddr *url.URL
	httpClient *http.Client
	username   string
	password   string
}

func NewCubbyClient(serverAddr string) (*CubbyClient, error) {
	parsedAddr, err := url.Parse(serverAddr)
	if err != nil {
		return nil, err
	}
	return &CubbyClient{
		username:   os.Getenv("CUBBY_USERNAME"),
		password:   os.Getenv("CUBBY_PASSWORD"),
		serverAddr: parsedAddr,
		httpClient: &http.Client{}}, nil
}

func (c *CubbyClient) keyUrlString(key string) string {
	return c.serverAddr.JoinPath(key).String()
}

func (c *CubbyClient) validate(resp *http.Response, err error) (*http.Response, error) {
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status code %v", resp.StatusCode)
	}
	return resp, nil
}

func (c *CubbyClient) NewRequest(method, key string, body io.Reader) (*http.Request, error) {
	request, err := http.NewRequest(method, c.keyUrlString(key), body)
	if err != nil {
		return nil, err
	}
	request.SetBasicAuth(c.username, c.password)
	return request, nil
}

func (c *CubbyClient) Get(key string) (string, error) {
	request, err := c.NewRequest(http.MethodGet, key, nil)
	if err != nil {
		return "", err
	}
	resp, err := c.validate(c.httpClient.Do(request))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(bodyBytes), nil
}

func (c *CubbyClient) Put(key, value string) error {
	request, err := c.NewRequest(http.MethodPost, key, strings.NewReader(value))
	if err != nil {
		return err
	}
	_, err = c.validate(c.httpClient.Do(request))
	return err
}

func (c *CubbyClient) Remove(key string) error {
	request, err := c.NewRequest(http.MethodDelete, key, nil)
	if err != nil {
		return err
	}
	_, err = c.validate(c.httpClient.Do(request))
	return err
}

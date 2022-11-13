package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type CubbyClient struct {
	serverAddr *url.URL
	httpClient *http.Client
}

func NewCubbyClient(serverAddr string) (*CubbyClient, error) {
	parsedAddr, err := url.Parse(serverAddr)
	if err != nil {
		return nil, err
	}
	return &CubbyClient{serverAddr: parsedAddr, httpClient: &http.Client{}}, nil
}

func (c *CubbyClient) keyUrlString(key string) string {
	return c.serverAddr.JoinPath(key).String()
}

func (c *CubbyClient) validate(resp *http.Response, err error) (*http.Response, error) {
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Request failed with status code %v", resp.StatusCode)
	}
	return resp, nil
}

func (c *CubbyClient) Get(key string) (string, error) {
	resp, err := c.validate(c.httpClient.Get(c.keyUrlString(key)))
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
	_, err := c.validate(c.httpClient.Post(c.keyUrlString(key), "", strings.NewReader(value)))
	return err
}

func (c *CubbyClient) Remove(key string) error {
	request, err := http.NewRequest(http.MethodDelete, c.keyUrlString(key), nil)
	if err != nil {
		return err
	}
	_, err = c.validate(c.httpClient.Do(request))
	return err
}

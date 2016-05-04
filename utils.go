package main

import (
	"io"
	"io/ioutil"
)

func respClose(body io.ReadCloser) error {
	if body == nil {
		return nil
	}
	if _, err := io.Copy(ioutil.Discard, body); err != nil {
		return err
	}
	return body.Close()
}

func contains(s []string, str string) bool {
	for _, a := range s {
		if a == str {
			return true
		}
	}
	return false
}

func panicIf(err error) {
	if err != nil {
		panic(err)
	}
}

func getClientIP(ip string) string {
	if len(ip) > 0 && ip == "::1" {
		return "127.0.0.1"
	}
	return ip
}

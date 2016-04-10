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

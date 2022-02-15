/*
Copyright 2022 The Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package util

import (
	"math/rand"
	"net"
	"net/url"
	"strconv"
	"time"
)

// RandomString returns a random alphanumeric string.
func RandomString(n int) string {
	charset := "0123456789abcdefghijklmnopqrstuvwxyz"
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	result := make([]byte, n)
	for i := range result {
		result[i] = charset[rnd.Intn(len(charset))]
	}
	return string(result)
}

func SplitHostPort(rawURL string) (string, int32, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", -1, err
	}

	host, strport, err := net.SplitHostPort(u.Host)
	if err != nil {
		return "", -1, err
	}

	port, err := strconv.Atoi(strport)
	if err != nil {
		return "", -1, err
	}

	return host, int32(port), nil
}

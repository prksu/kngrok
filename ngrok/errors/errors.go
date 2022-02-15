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

package errors

import (
	"errors"
	"fmt"
	"net/http"
)

type Error struct {
	Code       int         `json:"error_code"`
	StatusCode int         `json:"status_code"`
	Message    string      `json:"msg"`
	Details    interface{} `json:"details"`
}

func (err Error) Error() string {
	return fmt.Sprintf("ngrok agent api error: %s - code: %d - see https://ngrok.com/docs/errors for more details", err.Message, err.Code)
}

func IsNotFound(err error) bool {
	if nerr := Error(Error{}); errors.As(err, &nerr) {
		return nerr.StatusCode == http.StatusNotFound
	}

	return false
}

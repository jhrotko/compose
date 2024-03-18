/*
   Copyright 2020 Docker Compose CLI authors

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

package utils

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// StringContains check if an array contains a specific value
func StringContains(array []string, needle string) bool {
	for _, val := range array {
		if val == needle {
			return true
		}
	}
	return false
}

// StringToBool converts a string to a boolean ignoring errors
func StringToBool(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "y" {
		return true
	}
	b, _ := strconv.ParseBool(s)
	return b
}

func getEnvStr(key string) (string, error) {
	v := os.Getenv(key)
	if v == "" {
		return v, fmt.Errorf("env var not defined")
	}
	return v, nil
}

func GetEnvBool(key string) (bool, error) {
	s, err := getEnvStr(key)
	if err != nil {
		return false, err
	}
	v, err := strconv.ParseBool(s)
	if err != nil {
		return false, err
	}
	return v, nil
}

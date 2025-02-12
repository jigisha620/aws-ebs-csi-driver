// Copyright 2024 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the 'License');
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an 'AS IS' BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package template

import (
	"errors"
	"fmt"
	"html/template"
	"strings"
)

// Disable functions.
func html(...interface{}) (string, error) {
	return "", errors.New("cannot call 'html' function")
}

func js(...interface{}) (string, error) {
	return "", errors.New("cannot call 'js' function")
}

func call(...interface{}) (string, error) {
	return "", errors.New("cannot call 'call' function")
}

func urlquery(...interface{}) (string, error) {
	return "", errors.New("cannot call 'urlquery' function")
}

func contains(arg1, arg2 string) bool {
	return strings.Contains(arg2, arg1)
}

func substring(start, end int, arg string) string {
	if start < 0 {
		return arg[:end]
	}

	if end < 0 || end > len(arg) {
		return arg[start:]
	}

	return arg[start:end]
}

func field(delim string, idx int, arg string) (string, error) {
	w := strings.Split(arg, delim)
	if idx >= len(w) {
		return "", fmt.Errorf("extractWord: cannot index into split string; index = %d, length = %d", idx, len(w))
	}
	return w[idx], nil
}

func index(arg1, arg2 string) int {
	return strings.Index(arg2, arg1)
}

func lastIndex(arg1, arg2 string) int {
	return strings.LastIndex(arg2, arg1)
}

func newFuncMap() template.FuncMap {
	return template.FuncMap{
		"html":      html,
		"js":        js,
		"call":      call,
		"urlquery":  urlquery,
		"contains":  contains,
		"toUpper":   strings.ToUpper,
		"toLower":   strings.ToLower,
		"substring": substring,
		"field":     field,
		"index":     index,
		"lastIndex": lastIndex,
	}
}

// Copyright (c) 2018 Chef Software Inc. and/or applicable contributors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package broker

import (
	"math/rand"
	"time"
)

// Inspired from: https://stackoverflow.com/a/22892986/1773961

var letters = []rune("abcdefghijklmnopqrstuvwxyz1234567890")

func init() {
	rand.Seed(time.Now().Unix())
}

func randSeq(n int) string {
	buffer := make([]rune, n)
	for i := range buffer {
		buffer[i] = letters[rand.Intn(len(letters))]
	}
	return string(buffer)
}

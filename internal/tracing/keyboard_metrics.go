/*
   Copyright 2024 Docker Compose CLI authors

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

package tracing

type KeyboardMetrics struct {
	enabled          bool
	command          []Command
	commandAvailable []Command
}

type Command string

const (
	GUI   Command = "gui"
	WATCH Command = "watch"
	DEBUG Command = "debug"
)

func NewKeyboardMetrics(isDockerDesktopActive, isWatchConfigured bool) KeyboardMetrics {
	metrics := KeyboardMetrics{
		enabled:          true,
		commandAvailable: []Command{},
	}
	if isDockerDesktopActive {
		metrics.commandAvailable = append(metrics.commandAvailable, GUI)
	}
	if isWatchConfigured {
		metrics.commandAvailable = append(metrics.commandAvailable, WATCH)
	}
	return metrics
}

func (metrics *KeyboardMetrics) RegisterCommand(command Command) {
	metrics.command = append(metrics.command, command)
}

func CommandSliceToString(lst []Command) []string {
	result := []string{}
	for _, c := range lst {
		result = append(result, string(c))
	}
	return result
}

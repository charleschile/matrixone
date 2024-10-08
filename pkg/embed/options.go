// Copyright 2021-2024 Matrix Origin
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package embed

func WithConfigs(
	configs []string,
) Option {
	return func(c *cluster) {
		c.files = configs
	}
}

func WithPreStart(
	f func(ServiceOperator),
) Option {
	return func(c *cluster) {
		c.options.preStart = f
	}
}

func WithCNCount(
	cn int,
) Option {
	return func(c *cluster) {
		c.options.cn = cn
	}
}

func WithTesting() Option {
	return func(c *cluster) {
		c.options.testing = true
	}
}

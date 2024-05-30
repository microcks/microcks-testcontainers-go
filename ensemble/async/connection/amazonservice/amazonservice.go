/*
 * Copyright The Microcks Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
package amazonservice

// Connection represents an Amazon Service connection settings.
type Connection struct {
	// Region represents a region.
	Region string

	// EndpointOverride represents an endpoint override.
	EndpointOverride string

	// AccessKey represents an access key.
	accessKey string

	// SccessKey represents a secret key.
	SecretKey string
}

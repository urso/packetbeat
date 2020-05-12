// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package v2

import (
	"github.com/elastic/beats/v7/libbeat/common"
)

// Registry is a collection of extensions, that can consist of
// other registries and plugins.
type Registry struct {
	plugins map[string]Plugin
	subs    []*Registry
}

// Addon marks types that can be added to a Registry instance. It is implemented
// by Plugin and Registry only.
type Addon interface {
	addToRegistry(*Registry)
}

var _ Addon = (*Registry)(nil)

// NewRegistry creates a new registry from the given registries and plugins.
func NewRegistry(extensions ...Addon) *Registry {
	r := &Registry{}
	for _, e := range extensions {
		r.Add(e)
	}
	return r
}

func (c *Registry) addToRegistry(parent *Registry) {
	parent.subs = append(parent.subs, c)
}

// Add adds an existing registry or plugin.
func (r *Registry) Add(e Addon) {
	e.addToRegistry(r)
}

// Names returns a sorted list of known plugin names
func (r *Registry) Names() []string {
	uniq := common.StringSet{}
	r.each(func(p Plugin) bool {
		uniq.Add(p.Name)
		return true
	})
	return uniq.ToSlice()
}

// Each iterates over all known plugins accessible using this registry.
// The iteration stops when fn return false.
func (r *Registry) Each(fn func(Plugin) (cont bool)) {
	r.each(func(p Plugin) bool { return fn(p) })
}

func (r *Registry) each(fn func(Plugin) bool) bool {
	// Note: order of Find and each should be in the same order. Direct plugins
	// first followed by sub-registries.

	for _, p := range r.plugins {
		if !fn(p) {
			return false
		}
	}

	for _, sub := range r.subs {
		if !sub.each(fn) {
			return false
		}
	}
	return true
}

// Find searches for an existing extension for the given name. It returns
// an error if the extension does not exist.
func (c *Registry) Find(name string) (Plugin, error) {
	plugin, ok := c.find(name)
	if !ok {
		return plugin, &LoaderError{Name: name, Reason: ErrUnknown}
	}
	return plugin, nil
}

// Find returns the first Plugin matching the given name.
func (r *Registry) find(name string) (Plugin, bool) {
	// Note: order of Find and each should be in the same order. Direct plugins
	// first followed by sub-registries.

	if p, ok := r.plugins[name]; ok {
		return p, true
	}

	for _, sub := range r.subs {
		if p, ok := sub.find(name); ok {
			return p, ok
		}
	}
	return Plugin{}, false
}

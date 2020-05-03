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

package exclinput

import (
	"strings"
	"sync"
	"time"

	"github.com/elastic/beats/v7/libbeat/common/atomic"
	"github.com/elastic/beats/v7/libbeat/common/transform/typeconv"
	"github.com/elastic/beats/v7/libbeat/logp"
	"github.com/elastic/beats/v7/libbeat/statestore"
	"github.com/elastic/go-concert"
	"github.com/elastic/go-concert/unison"
	"github.com/urso/sderr"
)

// store encapsulates the persistent store and the in memory state store, that
// can be ahead of the the persistent store.
// The store lifetime is managed by a reference counter. Once all owners (the
// session, and the resource cleaner) have dropped ownership, backing resources
// will be released and closed.
type store struct {
	log             *logp.Logger
	refCount        concert.RefCount
	persistentStore *statestore.Store
	ephemeralStore  *states
}

// states stores resource states in memory. When a cursor for an input is updated,
// it's state is updated first. The entry in the persistent store 'follows' the internal state.
// As long as a resources stored in states is not 'Finished', the in memory
// store is assumed to be ahead (in memory and persistent state are out of
// sync).
type states struct {
	mu    sync.Mutex
	table map[string]*resource
}

type resource struct {
	// pending counts the number of Inputs and outstanding registry updates.
	// as long as pending is > 0 the resource is in used and must not be garbage collected.
	pending atomic.Uint64

	// lock guarantees only one input create updates for this entry
	lock unison.Mutex

	// stored indicates that the state is available in the registry file. It is false for new entries.
	stored bool

	// internalInSync is true if all 'Internal' metadata like TTL or update timestamp are in sync.
	// Normally resources are added when being created. But if operations failed we will retry inserting
	// them on each update operation until we eventually succeeded
	internalInSync bool

	// key of the resource as used for the registry.
	key string

	// state contains the cursor state and additional meta-data that is used by the input manager
	// to track the resource and implement garbage collection support on.
	state state
}

type (
	// state represents the full document as it is stored in the registry.
	//
	// The `Internal` namespace contains fields that are used by the input manager
	// for internal management tasks that are required to be persisted between restarts.
	//
	// The `Cursor` namespace is used to store the cursor information that are
	// required to continue processing from the last known position. Cursor
	// updates in the registry file are only executed after events have been
	// ACKed by the outputs. Therefore the cursor MUST NOT include any
	// information that are require to identify/track the source we are
	// collecting from.
	state struct {
		Internal stateInternal
		Cursor   interface{}
	}

	stateInternal struct {
		TTL     time.Duration
		Updated time.Time
	}

	stateInternalTTL     struct{ TTL time.Duration }
	stateInternalUpdated struct{ Updated time.Time }

	// registryStateUpdate is used to only transfer/update fields in the registry
	// that need to be updated when the pending update operations is finally
	// serialized.  Meta data that identify the cursor (like the key) must not be
	// updated via the ACK handling.
	registryStateUpdateCursor struct {
		Internal stateInternalUpdated
		Cursor   interface{}
	}

	registryStateUpdateInternal struct {
		Internal stateInternal
	}

	registryStateInsert struct {
		Internal stateInternalTTL
	}
)

func openStore(log *logp.Logger, statestore StateStore, prefix string) (*store, error) {
	ok := false

	persistentStore, err := statestore.Access()
	if err != nil {
		return nil, err
	}
	defer ifNotOK(&ok, func() { persistentStore.Close() })

	states, err := readStates(log, persistentStore, prefix)
	if err != nil {
		return nil, err
	}

	ok = true
	return &store{
		log: log,
		refCount: concert.RefCount{
			Action: func(_ error) {
				if err := persistentStore.Close(); err != nil {
					log.Errorf("Closing registry store did report an error: %+v", err)
				}
			},
		},
		persistentStore: persistentStore,
		ephemeralStore:  states,
	}, nil
}

func (s *store) Retain()  { s.refCount.Retain() }
func (s *store) Release() { s.refCount.Release() }

func (s *store) Find(key string, create bool) *resource {
	return s.ephemeralStore.Find(key, create)
}

func (s *store) UpdateInternal(resource *resource) {
	data := resource.state.Internal
	if data.Updated.IsZero() {
		data.Updated = time.Now()
	}

	err := s.persistentStore.Update(func(tx *statestore.Tx) error {
		return tx.Update(statestore.Key(resource.key), registryStateUpdateInternal{
			Internal: data,
		})
	})
	if err != nil {
		s.log.Errorf("Failed to update resource management fields for '%v'", resource.key)
		resource.internalInSync = false
	} else {
		resource.stored = true
		resource.internalInSync = true
	}
}

func (s *store) UpdateCursor(resource *resource, timestamp time.Time, updates interface{}) {
	updateCommand := registryStateUpdateCursor{
		Internal: stateInternalUpdated{Updated: timestamp},
		Cursor:   updates,
	}

	key := statestore.Key(resource.key)
	err := s.persistentStore.Update(func(tx *statestore.Tx) error {
		if !resource.internalInSync {
			internalUpdCommand := registryStateUpdateInternal{Internal: resource.state.Internal}
			if err := tx.Update(key, internalUpdCommand); err != nil {
				return err
			}
		}
		return tx.Update(key, updateCommand)
	})
	if err != nil {
		s.log.Errorf("Failed to update state in the registry for '%v'", key)
	} else {
		resource.internalInSync = true
		resource.stored = true
	}
}

func (s *store) Migrate(resource *resource, value interface{}) error {
	var tmp interface{}
	if err := typeconv.Convert(&tmp, value); err != nil {
		return sderr.Wrap(err, "failed to serialized state")
	}

	err := s.persistentStore.Update(func(tx *statestore.Tx) error {
		return tx.Set(statestore.Key(resource.key), value)
	})
	if err != nil {
		return sderr.Wrap(err, "failed to set key %{key} to new migrated value: %v", resource.key, value)
	}

	resource.SetCursor(tmp)
	return nil
}

func (s *states) Find(key string, create bool) *resource {
	s.mu.Lock()
	defer s.mu.Unlock()

	if resource := s.table[key]; resource != nil {
		resource.Retain()
		return resource
	}

	if !create {
		return nil
	}

	// resource is owned by table(session) and input that uses the resource.
	resource := &resource{
		stored: false,
		key:    key,
		lock:   unison.MakeMutex(),
		state:  state{},
	}
	s.table[key] = resource
	resource.Retain()
	return resource
}

func (e *resource) IsNew() bool {
	return e.state.Cursor == nil
}

// Retain is used to indicate that 'resource' gets an additional 'owner'.
// Owners of an resource can be active inputs or pending update operations
// not yet written to disk.
func (e *resource) Retain() { e.pending.Inc() }

// Release reduced the owner ship counter of the resource.
func (e *resource) Release() { e.pending.Dec() }

// Finished returns true if the resource is not in use and if there are no pending updates
// that still need to be written to the registry.
func (e *resource) Finished() bool { return e.pending.Load() == 0 }

// Unlock removes the exclusive access to the resource and gives up ownership.
// The input must not use the resource anymore after 'unlock' Only pending update operations
// will continue to excert ownership.
func (e *resource) Unlock() {
	e.lock.Unlock()
	e.Release()
}

func (e *resource) UnpackCursor(to interface{}) error {
	return typeconv.Convert(to, e.state.Cursor)
}

func (e *resource) SetCursor(c interface{}) {
	e.state.Cursor = c
}

func (e *resource) UpdateCursor(val interface{}) error {
	return typeconv.Convert(&e.state.Cursor, val)
}

func readStates(log *logp.Logger, store *statestore.Store, prefix string) (*states, error) {
	keyPrefix := prefix + "::"
	states := &states{
		table: map[string]*resource{},
	}

	// load 'local' states into memory
	err := store.View(func(tx *statestore.Tx) error {
		return tx.Each(func(k statestore.Key, dec statestore.ValueDecoder) (bool, error) {
			if !strings.HasPrefix(string(k), keyPrefix) {
				return true, nil
			}

			var st state
			if err := dec.Decode(&st); err != nil {
				log.Errorf("Failed to read regisry state for '%v', cursor state will be ignored. Error was: %+v",
					k, err)
				return true, nil
			}

			resource := &resource{
				key:    string(k),
				stored: true,
				lock:   unison.MakeMutex(),
				state:  st,
			}
			states.table[resource.key] = resource

			return true, nil
		})
	})

	if err != nil {
		return nil, err
	}
	return states, nil
}

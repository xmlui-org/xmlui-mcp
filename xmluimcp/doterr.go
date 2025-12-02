// MIT License
//
// Copyright (c) 2025 Mike Schinkel <mike@newclarity.net>
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

// Package xmluimcp provides a tiny, stdlib-only way to attach structured metadata
// and sentinels to errors while staying fully composable with the Go standard
// library. The model is:
//
//   - Each function builds an entry with New(...) passing an optional trailing cause:
//     return doterr.New(ErrRepo, "key", val, cause) // cause last
//
//   - With(...) is a flexible convenience for same-function enrichment. It can:
//
//   - Enrich the rightmost doterr entry within an existing joined error, or
//
//   - Join a new entry if no doterr entry exists, and
//
//   - (Optionally) treat a final trailing error as the cause and join it last.
//
//   - Combine([]error) bundles independent failures into a single error that unwraps
//     to its members, preserving order.
package xmluimcp

import (
	"errors"
	"fmt"
	"math/rand"
	"strings"
)

// KV represents a key/value metadata pair. Keys are preserved in
// insertion order, and values may be of any type.
type KV interface {
	Key() string
	Value() any
}

// Sentinel errors for validation failures
var (
	ErrMissingSentinel     = errors.New("missing required sentinel error")
	ErrTrailingKey         = errors.New("trailing key without value")
	ErrMisplacedError      = errors.New("error in wrong position")
	ErrInvalidArgumentType = errors.New("invalid argument type")
	ErrOddKeyValueCount    = errors.New("odd number of key-value arguments")
	ErrCrossPackageError   = errors.New("error from different doterr package")
	ErrFailedTypeAssertion = errors.New("failed type assertion")
)

// NewErr builds a standalone structured entry (no primary cause inside).
// Accepted parts:
//   - error         — sentinel/tag (required: at least one, must be first)
//   - KV{Key,Value} — explicit key/value
//   - "key", value  — implicit pair (value can be any type, including error)
//   - error         — optional trailing cause (joined last via errors.Join)
//
// Pattern: one or more sentinels (error), then zero or more key-value pairs,
// then optional trailing cause (error). After the first string key, all
// remaining args must form valid pairs, except for an optional final error.
// Returns nil if no meaningful parts are provided after validation.
// Returns a validation error joined with the partial entry if validation fails.
func NewErr(parts ...any) error {
	// Separate optional trailing cause from the parts
	coreParts, cause := extractTrailingCause(parts)

	if validationErr := validateNewParts(coreParts); validationErr != nil {
		// Return validation error joined as first error
		var e entry
		e.id = uniqueId
		appendEntry(&e, coreParts...)
		if e.empty() {
			return validationErr
		}
		return errors.Join(validationErr, e)
	}

	var e entry
	e.id = uniqueId
	appendEntry(&e, coreParts...)
	if e.empty() {
		return cause // if we only had a cause, return it
	}

	// Join entry with optional cause (cause last)
	if cause != nil {
		return errors.Join(e, cause)
	}
	return e
}

// WithErr is a flexible enrichment helper. Typical uses:
//
//	// Enrich an existing composite error (err may be an errors.Join tree):
//	err = doterr.With(err, "Foo", 10)
//
//	// Build an entry and join a trailing cause in one shot:
//	err = doterr.With("endpoint", ep, ErrTemplate, cause) // 'cause' is last
//
// Behavior:
//
//  1. If the FIRST arg is an error, it is treated as the base error to enrich:
//     • If it is a doterr entry, merge KVs/sentinels into that entry.
//     • If it is a multi-unwrap (errors.Join tree), find the RIGHTMOST
//     doterr entry, merge into it, and rebuild preserving order.
//     • If no doterr entry is found, a new entry will be joined in (see step 3).
//
//  2. After consuming the base (if present), if the LAST remaining arg is an error,
//     it is treated as the CAUSE and joined LAST.
//
//  3. The remaining middle args (if any) are collected into an entry. If we
//     enriched an existing doterr entry in step 1, that merged entry is used;
//     otherwise, a fresh entry is created. If there is a trailing CAUSE from step 2,
//     the result is errors.Join(entry, cause). If there is no cause, the entry is returned.
//
// Note: For inter-function composition, prefer New() with trailing cause:
//
//	return doterr.New(ErrRepo, "key", val, cause) // cause last
func WithErr(parts ...any) error {
	if len(parts) == 0 {
		return nil
	}

	i, j := 0, len(parts)-1

	// Optional base (first arg)
	var baseErr error
	firstErr, ok := parts[i].(error)
	if ok {
		baseErr = checkCrossPackage(firstErr)
		i++
	}

	// Optional cause (last arg)
	var cause error
	if j >= i {
		lastErr, ok := parts[j].(error)
		if ok {
			cause = checkCrossPackage(lastErr)
			j--
		}
	}

	// Middle segment are the metadata/sentinels to apply.
	middle := parts[i : j+1]

	// No base error: build entry from middle, then (if present) join cause LAST.
	if baseErr == nil {
		return handleCause(buildEntry(middle...), cause)
	}

	// Have a base error: try to enrich rightmost entry or join a fresh entry.
	err := buildErr(baseErr, middle)

	// Now handle the cause
	return handleCause(err, cause)

}

// CombineErrs bundles a slice of errors into a single composite error that unwraps
// to its members. Order is preserved and nils are skipped. Returns nil for an
// empty/fully-nil slice, or the sole error when there is exactly one.
func CombineErrs(errs []error) error {
	filtered := make([]error, 0, len(errs))
	for _, e := range errs {
		if e != nil {
			filtered = append(filtered, e)
		}
	}
	switch len(filtered) {
	case 0:
		return nil
	case 1:
		return filtered[0]
	default:
		return combined{errs: filtered}
	}
}

// ErrMeta returns the key/value pairs stored on a doterr entry.
// If err is a doterr entry, returns its metadata.
// If err is a joined error (has Unwrap() []error), scans immediate children
// left-to-right and returns metadata from the first doterr entry found.
// Otherwise returns nil.
// The returned slice preserves insertion order and is a copy.
func ErrMeta(err error) []KV {
	var ok bool
	var e, ce entry
	var cePtr *entry

	// Case (a): err is an entry → return its metadata
	//goland:noinspection GoTypeAssertionOnErrors,DuplicatedCode
	e, ok = err.(entry)
	if ok {
		out := make([]KV, len(e.kvs))
		for i, pair := range e.kvs {
			out[i] = pair
		}
		return out
	}

	// Case (b): err is a join (multi-unwrap) → scan immediate children left-to-right
	type unwrapper interface{ Unwrap() []error }
	u, ok := err.(unwrapper)
	if !ok {
		return nil
	}
	children := u.Unwrap()
	for _, child := range children {
		//goland:noinspection GoTypeAssertionOnErrors
		ce, ok = child.(entry)
		if !ok {
			//goland:noinspection GoTypeAssertionOnErrors
			cePtr, ok = child.(*entry)
			if ok {
				ce = *cePtr
			}
		}
		if ok {
			out := make([]KV, len(ce.kvs))
			for i, pair := range ce.kvs {
				out[i] = pair
			}
			return out
		}
	}

	return nil
}

// ErrValue extracts a single metadata value by key with type safety.
// Returns the value and true if found and the value is of type T.
// Returns the zero value of T and false if not found or type mismatch.
//
// Example:
//
//	status, ok := doterr.ErrValue[int](err, "http_status")
//	if ok {
//	    fmt.Printf("Status: %d\n", status)
//	}
//
//	name, ok := doterr.ErrValue[string](err, "parameter_name")
func ErrValue[T any](err error, key string) (T, bool) {
	var zero T
	kvs := ErrMeta(err)
	if kvs == nil {
		return zero, false
	}

	for _, pair := range kvs {
		if pair.Key() == key {
			if val, ok := pair.Value().(T); ok {
				return val, true
			}
			return zero, false
		}
	}
	return zero, false
}

// Errors returns the errors stored on a doterr entry.
// If err is a doterr entry, returns its errors.
// If err is a joined error (has Unwrap() []error), scans immediate children
// left-to-right and returns errors from the first doterr entry found.
// Otherwise returns nil.
// The returned slice preserves insertion order and is a copy.
//
// Note: These errors may be sentinel errors (e.g., ErrRepo), custom error types
// (e.g., *rfc9457.Error), or any other error type stored in the entry.
func Errors(err error) []error {
	// Case (a): err is an entry → return its errors
	//goland:noinspection GoTypeAssertionOnErrors
	e, ok := err.(entry)
	if ok {
		out := make([]error, len(e.errors))
		copy(out, e.errors)
		return out
	}

	// Case (b): err is a join (multi-unwrap) → scan immediate children left-to-right
	type unwrapper interface{ Unwrap() []error }
	u, ok := err.(unwrapper)
	if !ok {
		return nil
	}
	children := u.Unwrap()
	for _, child := range children {
		//goland:noinspection GoTypeAssertionOnErrors
		if ce, ok := child.(entry); ok {
			out := make([]error, len(ce.errors))
			copy(out, ce.errors)
			return out
		}
	}

	return nil
}

// FindErr walks an error tree (including errors.Join trees)
// and returns the first match for target (via errors.As).
func FindErr[T error](err error) (out T, ok bool) {
	var t T
	if errors.As(err, &t) {
		return t, true
	}
	return out, false
}

func AppendErr(errs []error, err error) []error {
	if err == nil {
		return errs
	}
	return append(errs, err)
}

//--------------------------------
// Unexported implementation types
//--------------------------------

// kv is the internal implementation of the KV interface.
type kv struct {
	k string
	v any
}

func (p kv) Key() string { return p.k }
func (p kv) Value() any  { return p.v }

var uniqueId = rand.Int()

// entry represents one function's contribution to an error chain.
// Each function creates one entry with errors (sentinels, custom typed errors) and metadata.
// It implements error and Unwrap() []error.
type entry struct {
	id     int     // Unique ID
	errors []error // sentinels, custom typed errors (NOT the primary cause)
	kvs    []kv    // metadata in insertion order
}

func newEntry(errors []error, kvs []kv) *entry {
	return &entry{
		id:     uniqueId,
		errors: errors,
		kvs:    kvs,
	}
}

func (e entry) Error() string {
	if len(e.errors) == 0 && len(e.kvs) == 0 {
		return "doterr{}"
	}

	var parts []string

	// Include sentinel errors first
	for _, err := range e.errors {
		parts = append(parts, err.Error())
	}

	// Then include metadata
	if len(e.kvs) > 0 {
		meta := "meta:"
		for _, pair := range e.kvs {
			meta += " " + fmt.Sprintf("%s=%v", pair.k, pair.v)
		}
		parts = append(parts, meta)
	}

	return strings.Join(parts, "; ")
}

func (e entry) Unwrap() []error {
	if len(e.errors) == 0 {
		return nil
	}
	cp := make([]error, len(e.errors))
	copy(cp, e.errors)
	return cp
}

func (e entry) empty() bool { return len(e.errors) == 0 && len(e.kvs) == 0 }

func appendEntry(e *entry, parts ...any) {
	for i := 0; i < len(parts); {
		switch v := parts[i].(type) {
		case KV:
			// Convert interface to internal kv
			e.kvs = append(e.kvs, kv{k: v.Key(), v: v.Value()})
			i++
		case string:
			if i+1 < len(parts) {
				e.kvs = append(e.kvs, kv{k: v, v: parts[i+1]})
				i += 2
			} else {
				// Trailing key without value: skip it (validation should have caught this)
				i++
			}
		case error:
			if v != nil {
				e.errors = append(e.errors, v) // sentinel/tag
			}
			i++
		default:
			// Non-string, non-error, non-KV: skip it (validation should have caught this)
			i++
		}
	}
}

// combined implements a composite error for Combine().
type combined struct{ errs []error }

func (c combined) Error() string {
	var messages []string
	for _, err := range c.errs {
		if err != nil {
			messages = append(messages, err.Error())
		}
	}
	return strings.Join(messages, "\n")
}

func (c combined) Unwrap() []error {
	cp := make([]error, len(c.errs))
	copy(cp, c.errs)
	return cp
}

//------------------------
// Unexported helper funcs
//------------------------

// extractTrailingCause checks if the last element in parts is an error that should
// be treated as a trailing cause. Returns (cause, remaining parts).
// A trailing error is considered a cause if:
//   - It's the last element, AND
//   - It comes after at least one sentinel, AND
//   - It's not part of an incomplete key-value pair (would leave odd count)
func extractTrailingCause(parts []any) (_ []any, err error) {
	var lastIdx, sentinelCount, nonSentinelCount int
	var ok bool

	if len(parts) == 0 {
		goto end
	}

	lastIdx = len(parts) - 1
	err, ok = parts[lastIdx].(error)
	if !ok {
		goto end
	}

	// Count sentinels at the beginning (but don't count doterr.entry as sentinel)
	sentinelCount = 0
	for i := 0; i < len(parts); i++ {
		err, ok := parts[i].(error)
		if !ok {
			break
		}
		// doterr.entry should not count as a sentinel - it's a wrapped error
		//goland:noinspection GoTypeAssertionOnErrors
		_, ok = err.(entry)
		if ok {
			break
		}
		sentinelCount++
	}

	// If we only have errors (all sentinels), the last one is not a cause
	if sentinelCount == len(parts) {
		goto end
	}

	// Check if removing the last error would leave an odd number of non-sentinel args
	// (which would mean the error is actually a value for a key)
	nonSentinelCount = len(parts) - sentinelCount
	if (nonSentinelCount-1)%2 != 0 {
		// Removing last error leaves odd count - it's a value, not a cause
		goto end
	}

	parts = parts[:lastIdx]
end:
	// Last error is a trailing cause
	return parts, err
}

// validateNewParts validates that parts conforms to the expected pattern for NewErr:
//   - At least one sentinel error (required)
//   - Then zero or more KV or string key-value pairs
//   - After the first string key, remaining args must form valid pairs (even count)
//   - No non-string/non-error/non-KV values allowed
func validateNewParts(parts []any) error {
	if len(parts) == 0 {
		// Build entry manually to avoid recursion
		e := newEntry([]error{ErrMissingSentinel}, []kv{
			{k: "message", v: "doterr.New requires at least one sentinel error"},
		})
		return e
	}

	// Find where sentinels end (first non-error)
	sentinelCount := 0
	i := 0
	for i < len(parts) {
		if _, ok := parts[i].(error); ok {
			sentinelCount++
			i++
		} else {
			break
		}
	}

	// Must have at least one sentinel
	if sentinelCount == 0 {
		// Build entry manually to avoid recursion
		e := newEntry([]error{ErrMissingSentinel}, []kv{
			{k: "message", v: "doterr.New requires at least one sentinel error as the first argument"},
		})
		return e
	}

	// Validate remaining args are valid key-value pairs
	firstKeyIdx := -1
	for j := i; j < len(parts); j++ {
		switch v := parts[j].(type) {
		case KV:
			// Explicit KV is always valid
			continue
		case string:
			// Mark first key position if not yet found
			if firstKeyIdx == -1 {
				firstKeyIdx = j
			}
			// After first key, must have even number of remaining args
			if j+1 >= len(parts) {
				// Build entry manually to avoid recursion
				e := newEntry([]error{ErrTrailingKey}, []kv{
					{k: "key", v: v},
					{k: "position", v: j},
				})
				return e
			}
			j++ // Skip the value
		case error:
			// Errors after sentinels are not allowed
			// Build entry manually to avoid recursion
			e := newEntry([]error{ErrMisplacedError}, []kv{
				{k: "position", v: j},
				{k: "message", v: "errors must be first"},
			})
			return e
		default:
			// Non-string, non-error, non-KV values are not allowed
			// Build entry manually to avoid recursion
			e := newEntry([]error{ErrInvalidArgumentType}, []kv{
				{k: "type", v: fmt.Sprintf("%T", v)},
				{k: "position", v: j},
				{k: "message", v: "only error, KV, or string keys allowed"},
			})
			return e
		}
	}

	// If we found a first key, check that remaining count is even
	if firstKeyIdx >= 0 {
		remaining := len(parts) - firstKeyIdx
		if remaining%2 != 0 {
			// Build entry manually to avoid recursion
			e := newEntry([]error{ErrOddKeyValueCount}, []kv{
				{k: "position", v: firstKeyIdx},
				{k: "remaining_count", v: remaining},
			})
			return e
		}
	}

	return nil
}

// buildEntry creates an entry from parts without validation.
// Used internally by WithErr where sentinels are optional.
// Returns nil if parts are empty or result in an empty entry.
func buildEntry(parts ...any) error {
	if len(parts) == 0 {
		return nil
	}
	var e entry
	e.id = uniqueId
	appendEntry(&e, parts...)
	if e.empty() {
		return nil
	}
	return e
}

// handleCause inspects err and cause and, if cause is non-nil,
// returns errors.Join(err, cause) with the cause LAST.
func handleCause(err, cause error) error {
	if cause == nil {
		return err
	}
	if err == nil {
		return cause
	}
	// If we have a trailing cause, join it LAST.
	return errors.Join(err, cause)
}

// checkCrossPackage wraps an error with ErrCrossPackageError if it's an entry
// from a different doterr package (different uniqueId).
func checkCrossPackage(err error) error {
	//goland:noinspection GoTypeAssertionOnErrors
	e, isEntry := err.(entry)
	if isEntry && e.id != uniqueId {
		// Cross-package error detected - prepend sentinel
		crossPkgErr := buildEntry(ErrCrossPackageError, "package_id", e.id, "expected_id", uniqueId)
		return errors.Join(crossPkgErr, err)
	}
	return err
}

// buildErr tries to enrich the rightmost doterr entry inside baseErr.
// If none found, it joins a fresh entry (from middle) with baseErr,
// preserving baseErr's internals (including any existing cause).
func buildErr(baseErr error, middle []any) error {
	enriched, ok := enrichRightmost(baseErr, middle...)
	if ok {
		// Successfully merged into an existing doterr entry.
		return enriched
	}
	// No doterr entry found inside base; create a fresh entry and join it with base.
	e := buildEntry(middle...)
	if e != nil {
		// cause remains inside baseErr
		return errors.Join(e, baseErr)
	}
	return baseErr
}

// enrichRightmost tries to merge parts into a doterr entry that is either:
//
//	(a) the error itself, or
//	(b) one of the immediate children of a multi-unwrap (errors.Join) tree.
//
// It does NOT recurse deeper than one join level.
func enrichRightmost(err error, parts ...any) (error, bool) {
	// Case (a): err is an entry → merge directly.
	//goland:noinspection GoTypeAssertionOnErrors
	e, ok := err.(entry)
	if ok {
		tmp := e
		appendEntry(&tmp, parts...)
		return tmp, true
	}

	// Case (b): err is a join (multi-unwrap) → scan immediate children right-to-left.
	type unwrapper interface{ Unwrap() []error }
	u, ok := err.(unwrapper)
	if !ok {
		return err, false
	}
	children := u.Unwrap()
	if len(children) == 0 {
		return err, false
	}
	newKids := make([]error, len(children))
	copy(newKids, children)

	for i := len(newKids) - 1; i >= 0; i-- {
		//goland:noinspection GoTypeAssertionOnErrors
		e, ok := newKids[i].(entry)
		if ok {
			tmp := e
			appendEntry(&tmp, parts...)
			newKids[i] = tmp
			return errors.Join(newKids...), true
		}
		// NOTE: Deliberately NO recursion into nested joins.
	}

	// No doterr entry found at this level.
	return err, false
}

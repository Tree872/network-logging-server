// File: limiter.go
// Description: This file contains the implementation of a rate limiter for the network logging server.
package ratelimit

import (
	"sync"
	"time"
)

type bucket struct {
	startedAt time.Time
	count     int
}

type Limiter struct {
	maxMessages int
	window      time.Duration
	buckets     map[string]bucket
	lastCleanup time.Time
	mutex       sync.Mutex
}

func New(maxMessages int, window time.Duration) *Limiter {
	var limiter Limiter

	limiter.maxMessages = maxMessages
	limiter.window = window
	limiter.buckets = make(map[string]bucket)
	limiter.lastCleanup = time.Now()

	return &limiter
}

func (limiter *Limiter) Allow(clientIP string) bool {
	var now time.Time
	var clientBucket bucket
	var exists bool

	now = time.Now()

	limiter.mutex.Lock()
	defer limiter.mutex.Unlock()

	// Periodically remove expired IP entries to prevent unbounded growth.
	if now.Sub(limiter.lastCleanup) >= limiter.window {
		var ip string
		var existingBucket bucket

		for ip, existingBucket = range limiter.buckets {
			if now.Sub(existingBucket.startedAt) >= limiter.window {
				delete(limiter.buckets, ip)
			}
		}

		limiter.lastCleanup = now
	}

	clientBucket, exists = limiter.buckets[clientIP]

	if !exists || now.Sub(clientBucket.startedAt) >= limiter.window {
		clientBucket = bucket{
			startedAt: now,
			count:     0,
		}
	}

	if clientBucket.count >= limiter.maxMessages {
		limiter.buckets[clientIP] = clientBucket
		return false
	}

	clientBucket.count++
	limiter.buckets[clientIP] = clientBucket

	return true
}

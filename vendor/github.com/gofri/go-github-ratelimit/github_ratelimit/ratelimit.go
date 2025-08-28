package github_ratelimit

import (
	"context"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type SecondaryRateLimitWaiter struct {
	base       http.RoundTripper
	sleepUntil *time.Time
	lock       sync.RWMutex

	// limits
	totalSleepTime   time.Duration
	singleSleepLimit *time.Duration
	totalSleepLimit  *time.Duration

	// callbacks
	userContext           *context.Context
	onLimitDetected       OnLimitDetected
	onSingleLimitExceeded OnSingleLimitExceeded
	onTotalLimitExceeded  OnTotalLimitExceeded
}

func NewRateLimitWaiter(base http.RoundTripper, opts ...Option) (*SecondaryRateLimitWaiter, error) {
	if base == nil {
		base = http.DefaultTransport
	}

	waiter := SecondaryRateLimitWaiter{
		base: base,
	}
	applyOptions(&waiter, opts...)

	return &waiter, nil
}

func NewRateLimitWaiterClient(base http.RoundTripper, opts ...Option) (*http.Client, error) {
	waiter, err := NewRateLimitWaiter(base, opts...)
	if err != nil {
		return nil, err
	}

	return &http.Client{
		Transport: waiter,
	}, nil
}

// RoundTrip handles the secondary rate limit by waiting for it to finish before issuing new requests.
// If a request got a secondary rate limit error as a response, we retry the request after waiting.
// Issuing more requests during a secondary rate limit may cause a ban from the server side,
// so we want to prevent these requests, not just for the sake of cpu/network utilization.
// Nonetheless, there is no way to prevent subtle race conditions without completely serializing the requests,
// so we prefer to let some slip in case of a race condition, i.e.,
// after a retry-after response is received and before it is processed,
// a few other (parallel) requests may be issued.
func (t *SecondaryRateLimitWaiter) RoundTrip(request *http.Request) (*http.Response, error) {
	t.waitForRateLimit()

	resp, err := t.base.RoundTrip(request)
	if err != nil {
		return resp, err
	}

	secondaryLimit := parseSecondaryLimitTime(resp)
	if secondaryLimit == nil {
		return resp, nil
	}

	callbackContext := CallbackContext{
		Request:  request,
		Response: resp,
	}

	shouldRetry := t.updateRateLimit(*secondaryLimit, &callbackContext)
	if !shouldRetry {
		return resp, nil
	}

	return t.RoundTrip(request)
}

// waitForRateLimit waits for the cooldown time to finish if a secondary rate limit is active.
func (t *SecondaryRateLimitWaiter) waitForRateLimit() {
	t.lock.RLock()
	sleepTime := t.currentSleepTimeUnlocked()
	t.lock.RUnlock()

	time.Sleep(sleepTime)
}

// updateRateLimit updates the active rate limit and triggers user callbacks if needed.
// the rate limit is not updated if there is already an active rate limit.
// it never waits because the retry handles sleeping anyway.
// returns whether or not to retry the request.
func (t *SecondaryRateLimitWaiter) updateRateLimit(secondaryLimit time.Time, callbackContext *CallbackContext) bool {
	// quick check without the lock: maybe the secondary limit just passed
	if time.Now().After(secondaryLimit) {
		return true
	}

	t.lock.Lock()
	defer t.lock.Unlock()

	// check before update if there is already an active rate limit
	if t.currentSleepTimeUnlocked() > 0 {
		return true
	}

	// check if the secondary rate limit happened to have passed while we waited for the lock
	sleepTime := time.Until(secondaryLimit)
	if sleepTime <= 0 {
		return true
	}

	// do not sleep in case it is above the single sleep limit
	if t.singleSleepLimit != nil && sleepTime > *t.singleSleepLimit {
		t.triggerCallback(t.onSingleLimitExceeded, callbackContext, secondaryLimit)
		return false
	}

	// do not sleep in case it is above the total sleep limit
	if t.totalSleepLimit != nil && t.totalSleepTime+sleepTime > *t.totalSleepLimit {
		t.triggerCallback(t.onTotalLimitExceeded, callbackContext, secondaryLimit)
		return false
	}

	// a legitimate new limit
	t.sleepUntil = &secondaryLimit
	t.totalSleepTime += sleepTime
	t.triggerCallback(t.onLimitDetected, callbackContext, secondaryLimit)

	return true
}

func (t *SecondaryRateLimitWaiter) currentSleepTimeUnlocked() time.Duration {
	if t.sleepUntil == nil {
		return 0
	}
	return time.Until(*t.sleepUntil)
}

// parseSecondaryLimitTime parses the GitHub API response header,
// looking for the secondary rate limit as defined by GitHub API documentation.
// https://docs.github.com/en/rest/overview/resources-in-the-rest-api#secondary-rate-limits
func parseSecondaryLimitTime(resp *http.Response) *time.Time {
	if resp.StatusCode != http.StatusForbidden {
		return nil
	}

	if resp.Header == nil {
		return nil
	}

	if sleepUntil := parseRetryAfter(resp.Header); sleepUntil != nil {
		return sleepUntil
	}

	if sleepUntil := parseXRateLimitReset(resp.Header); sleepUntil != nil {
		return sleepUntil
	}

	return nil
}

// parseRetryAfter parses the GitHub API response header in case a Retry-After is returned.
func parseRetryAfter(header http.Header) *time.Time {
	retryAfterSeconds, ok := httpHeaderIntValue(header, "retry-after")
	if !ok || retryAfterSeconds <= 0 {
		return nil
	}

	// per GitHub API, the header is set to the number of seconds to wait
	sleepUntil := time.Now().Add(time.Duration(retryAfterSeconds) * time.Second)

	return &sleepUntil
}

// parseXRateLimitReset parses the GitHub API response header in case a x-ratelimit-reset is returned.
// to avoid handling primary rate limits (which are categorized),
// we only handle x-ratelimit-reset in case the primary rate limit is not reached.
func parseXRateLimitReset(header http.Header) *time.Time {
	if remaining, ok := httpHeaderIntValue(header, "x-ratelimit-remaining"); ok && remaining == 0 {
		// this is a primary rate limit; ignore it
		return nil
	}

	secondsSinceEpoch, ok := httpHeaderIntValue(header, "x-ratelimit-reset")
	if !ok || secondsSinceEpoch <= 0 {
		return nil
	}

	// per GitHub API, the header is set to the number of seconds since epoch (UTC)
	sleepUntil := time.Unix(secondsSinceEpoch, 0)

	return &sleepUntil
}

func httpHeaderIntValue(header http.Header, key string) (int64, bool) {
	val, ok := header[http.CanonicalHeaderKey(key)]
	if !ok || len(val) == 0 {
		return 0, false
	}
	asInt, err := strconv.ParseInt(val[0], 10, 64)
	if err != nil {
		return 0, false
	}
	return asInt, true
}

func (t *SecondaryRateLimitWaiter) triggerCallback(callback func(*CallbackContext), callbackContext *CallbackContext, newSleepUntil time.Time) {
	if callback == nil {
		return
	}

	callbackContext.RoundTripper = t
	callbackContext.UserContext = t.userContext
	callbackContext.SleepUntil = &newSleepUntil
	callbackContext.TotalSleepTime = &t.totalSleepTime

	callback(callbackContext)
}

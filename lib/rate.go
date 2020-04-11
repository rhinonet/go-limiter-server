package limiter

import (
	"time"
	"github.com/go-redis/redis"
)

type rediser interface {
	Eval(script string, keys []string, args ...interface{}) *redis.Cmd
	EvalSha(sha1 string, keys []string, args ...interface{}) *redis.Cmd
	ScriptExists(hashes ...string) *redis.BoolSliceCmd
	ScriptLoad(script string) *redis.StringCmd
}

type Limit struct {
	Rate   int
	Period time.Duration
	Burst  int
}

func PerSecond(rate int) *Limit {
	return &Limit{
		Rate:   rate,
		Period: time.Second,
		Burst:  rate,
	}
}

//------------------------

type Limiter struct {
	rdb rediser
}

func NewLimiter(rdb rediser) *Limiter {
	return &Limiter{
		rdb: rdb,
	}
}

func (l *Limiter) AllowN(key string, limit *Limit, n int) (*Result, error) {
	values := []interface{}{limit.Burst, limit.Rate, limit.Period.Seconds(), n}
	v, err := gcra.Run(l.rdb, []string{key}, values...).Result()
	if err != nil {
		return nil, err
	}

	values = v.([]interface{})

	res := &Result{
		Limit:      limit,
		Allowed:    values[0].(int64) == 0,
		Remaining:  int(values[1].(int64)),
	}
	return res, nil
}

type Result struct {
	// Limit is the limit that was used to obtain this result.
	Limit *Limit

	// Allowed reports whether event may happen at time now.
	Allowed bool

	// Remaining is the maximum number of requests that could be
	// permitted instantaneously for this key given the current
	// state. For example, if a rate limiter allows 10 requests per
	// second and has already received 6 requests for this key this
	// second, Remaining would be 4.
	Remaining int
}

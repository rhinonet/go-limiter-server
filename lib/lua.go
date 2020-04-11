package limiter

import "github.com/go-redis/redis"

var gcra = redis.NewScript(`
redis.replicate_commands()

local rate_limit_key = KEYS[1]
--bucket
local burst = ARGV[1]
--token num
local tokens = ARGV[2]
--seconds num
local seconds = ARGV[3]

local emission_interval = seconds / tokens
local r = tokens / seconds
local increment = emission_interval
local now = redis.call("TIME")
local jan_1_2017 = 1483228800
now = (now[1] - jan_1_2017) + (now[2] / 1000000)

local volume_key = "volume:"..rate_limit_key
local rate_key = "rate:" .. rate_limit_key
local w = redis.call("GET", volume_key)
if not w then
  w = burst
else
  w = tonumber(w)
end

local tat = redis.call("GET", rate_key)
if not tat then
  tat = now
else
  tat = tonumber(tat)
end

local limited

local wb = (now - tat) * r
wb = wb + w
w = math.min(wb, burst)
if w > 1 then 
    w = w - 1
    limited = 0
	redis.call("SET", volume_key, w)
	redis.call("SET", rate_key, now)
else
	limited = 1
end

return {limited, w}
`)

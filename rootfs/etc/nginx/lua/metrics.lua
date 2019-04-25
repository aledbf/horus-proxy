local configuration = require("configuration")

local _M = {}

local last_request_timestamp = ngx.now()

local metric_requests = prometheus:counter(
    "http_requests_total", "Number of HTTP requests", {"host", "status"})
local metric_latency = prometheus:histogram(
    "http_requests_duration_seconds", "HTTP request latency", {"host"})
local metric_connections = prometheus:gauge(
    "http_connections", "Number of HTTP connections", {"state"})
local metric_waiting_for_endpoint = prometheus:gauge(
    "http_requests_waiting_endpoint", "Info metric indicating if the proxy is waiting for pods")
local metric_last_request = prometheus:gauge(
    "http_requests_seconds_ago", "Number of seconds since the last connection")

function _M.collect()
  metric_connections:set(ngx.var.connections_reading, {"reading"})
  metric_connections:set(ngx.var.connections_waiting, {"waiting"})
  metric_connections:set(ngx.var.connections_writing, {"writing"})

  local seconds_from_last_request = math.ceil(ngx.now() - last_request_timestamp)
  metric_last_request:set(seconds_from_last_request)

  local waiting = 0
  if configuration.get_waiting_for_endpoints() then
    waiting = 1
  end

  metric_waiting_for_endpoint:set(waiting)

  prometheus:collect()
end

function _M.log()
  last_request_timestamp = ngx.now()

  metric_requests:inc(1, {ngx.var.server_name, ngx.var.status})
  metric_latency:observe(tonumber(ngx.var.request_time), {ngx.var.server_name})
end

return _M

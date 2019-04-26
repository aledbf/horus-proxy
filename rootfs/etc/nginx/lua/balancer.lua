local ngx_balancer = require("ngx.balancer")
local cjson = require("cjson.safe")
local configuration = require("configuration")
local round_robin = require("balancer.round_robin")

-- measured in seconds
-- for an Nginx worker to pick up the new list of upstream peers
-- it will take <the delay until controller POSTed the backend object to the Nginx endpoint> + BACKENDS_SYNC_INTERVAL
local BACKENDS_SYNC_INTERVAL = 1

local _M = {}
local balancers = {}

local function format_ipv6_endpoints(endpoints)
  local formatted_endpoints = {}
  for _, endpoint in ipairs(endpoints) do
    local formatted_endpoint = endpoint
    if not endpoint.address:match("^%d+.%d+.%d+.%d+$") then
      formatted_endpoint.address = string.format("[%s]", endpoint.address)
    end
    table.insert(formatted_endpoints, formatted_endpoint)
  end
  return formatted_endpoints
end

local function sync_backend(backend)
  if not backend.endpoints or #backend.endpoints == 0 then
    ngx.log(ngx.INFO, string.format("there is no endpoint for backend %s. Removing...", backend.name))
    balancers[backend.name] = nil
    configuration.set_endpoint_count(0)
    return
  end

  configuration.set_endpoint_count(#backend.endpoints)

  local implementation = round_robin
  local balancer = balancers[backend.name]

  if not balancer then
    balancers[backend.name] = implementation:new(backend)
    return
  end

  backend.endpoints = format_ipv6_endpoints(backend.endpoints)

  balancer:sync(backend)
end

local function sync_backends()
  local backends_data = configuration.get_backends_data()
  if not backends_data then
    balancers = {}
    return
  end

  local new_backends, err = cjson.decode(backends_data)
  if not new_backends then
    ngx.log(ngx.ERR, "could not parse backends data: ", err)
    return
  end

  local balancers_to_keep = {}
  for _, new_backend in ipairs(new_backends) do
    sync_backend(new_backend)
    balancers_to_keep[new_backend.name] = balancers[new_backend.name]
  end

  for backend_name, _ in pairs(balancers) do
    if not balancers_to_keep[backend_name] then
      balancers[backend_name] = nil
    end
  end
end

local function wait_for_balancer()
  local backend_name = ngx.var.proxy_upstream_name

  local balancer
  -- wait up to 5 minutes
  while true do
    balancer = balancers[backend_name]
    if not balancer then
      configuration.set_waiting_for_endpoints(true)
      ngx.log(ngx.DEBUG, "not upstream servers available in ", backend_name)
      ngx.sleep(math.random(3,7))
    else
      configuration.set_waiting_for_endpoints(false)
      break
    end
  end
end

local function get_balancer()
  local backend_name = ngx.var.proxy_upstream_name
  local balancer = balancers[backend_name]
  if not balancer then
    return
  end

  return balancer
end

function _M.init_worker()
  sync_backends() -- when worker starts, sync backends without delay
  local _, err = ngx.timer.every(BACKENDS_SYNC_INTERVAL, sync_backends)
  if err then
    ngx.log(ngx.ERR, string.format("error when setting up timer.every for sync_backends: %s", tostring(err)))
  end
end

function _M.rewrite()
  local balancer = get_balancer()
  if not balancer then
    ngx.status = ngx.HTTP_SERVICE_UNAVAILABLE
    return ngx.exit(ngx.status)
  end
end

function _M.balance()
  local balancer = get_balancer()
  if not balancer then
    return
  end

  local peer = balancer:balance()
  if not peer then
    ngx.log(ngx.WARN, "no peer was returned, balancer: " .. balancer.name)
    return
  end

  ngx_balancer.set_more_tries(1)

  local ok, err = ngx_balancer.set_current_peer(peer)
  if not ok then
    ngx.log(ngx.ERR, string.format("error while setting current upstream peer %s: %s", peer, err))
  end
end

function _M.log()
  local balancer = get_balancer()
  if not balancer then
    return
  end

  if not balancer.after_balance then
    return
  end

  balancer:after_balance()
end

function _M.not_found()
  ngx.status = ngx.HTTP_NOT_FOUND
  ngx.header.content_type = "application/json"
  ngx.say('{"error":"Not Found"}')
  return ngx.exit(ngx.HTTP_NOT_FOUND)
end

if _TEST then
  _M.sync_backend = sync_backend
end

function _M.wait_for_balancer()
  wait_for_balancer()
end

return _M

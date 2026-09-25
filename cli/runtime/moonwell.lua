-- Moonwell runtime prelude. Emitted at the top of the bundle's `do ... end` block.
-- Lua 5.3 compatible. Everything here is local to the bundle except the globals
-- `require`, `config` and `main`, which it replaces.
local __mw = {
  modules = {},
  loaded = {},
  lines = {},
  -- Set by minified builds: line numbers then say nothing about the .yue source.
  minified = false,
  hooks = { before_config = {}, on_config = {}, before_main = {}, on_main = {} },
}

function __mw.define(name, loader)
  __mw.modules[name] = loader
end

function __mw.require(name)
  local cached = __mw.loaded[name]
  if cached ~= nil then
    return cached
  end
  local loader = __mw.modules[name]
  if loader == nil then
    error("module '" .. tostring(name) .. "' is not in the bundle", 2)
  end
  local result = loader(name)
  if result == nil then
    result = true
  end
  __mw.loaded[name] = result
  return result
end

-- Maps an absolute war3map.lua line to "src/file.yue:<line>" ("src/file.yue" when minified), or nil outside modules.
local function map_line(line)
  local lines = __mw.lines
  for i = #lines, 1, -1 do
    local entry = lines[i]
    if line >= entry[1] then
      if line <= entry[2] then
        if __mw.minified then
          return entry[4]
        end
        return entry[4] .. ":" .. (line - entry[1] + 1)
      end
      return nil
    end
  end
  return nil
end

local function remap(line)
  local mapped = map_line(tonumber(line))
  if mapped ~= nil then
    return mapped .. ":"
  end
  return nil
end

function __mw.format_error(message)
  local text = tostring(message)
  text = text:gsub('%[string "war3map%.lua"%]:(%d+):', remap)
  text = text:gsub("war3map%.lua:(%d+):", remap)
  return text
end

local function handler(err)
  local text = tostring(err)
  if debug ~= nil and debug.traceback ~= nil then
    text = debug.traceback(text, 2)
  end
  return __mw.format_error(text)
end

function __mw.report(label, err)
  print("|cffff4040[moonwell] " .. label .. " failed:|r " .. tostring(err))
end

local function protect(label, fn, ...)
  local ok, err = xpcall(fn, handler, ...)
  if not ok then
    __mw.report(label, err)
  end
  return ok
end

local function run_hooks(phase)
  local list = __mw.hooks[phase]
  for i = 1, #list do
    protect(phase, list[i])
  end
end

function __mw.install()
  local original_config, original_main = config, main
  config = function()
    run_hooks("before_config")
    if original_config ~= nil then
      protect("config", original_config)
    end
    run_hooks("on_config")
  end
  main = function()
    run_hooks("before_main")
    if original_main ~= nil then
      protect("main", original_main)
    end
    run_hooks("on_main")
  end
end

function __mw.boot(entry)
  protect("load " .. entry, __mw.require, entry)
end

local function register(phase)
  return function(fn)
    if type(fn) ~= "function" then
      error("moonwell." .. phase .. " expects a function", 2)
    end
    local list = __mw.hooks[phase]
    list[#list + 1] = fn
  end
end

__mw.loaded["moonwell"] = {
  before_config = register("before_config"),
  on_config = register("on_config"),
  before_main = register("before_main"),
  on_main = register("on_main"),
  format_error = __mw.format_error,
}

require = __mw.require

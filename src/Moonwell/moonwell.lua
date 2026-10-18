-- Moonwell's Lua helpers, loaded after the core system. MoonwellOverrides(obj, name, baseType): whether a class
-- between obj's class and baseType's (a System.Type; its class table is [1]) defines name itself.
function MoonwellOverrides(obj, name, baseType)
  local base = baseType[1]
  local cls = getmetatable(obj)
  while cls ~= nil and cls ~= base do
    if rawget(cls, name) ~= nil then return true end
    cls = getmetatable(cls)
  end
  return false
end

-- MoonwellField(obj, id, kind): the value of an object field of obj's class, read from MoonwellObjectValues, which the
-- build appends to the script with the fields the map's code reads of each class it uses. Its getters in the objects'
-- library call it. kind is "value"; "levels", a table with get(level) and getCount(); "list", a table with getCount(),
-- Contains(T) and an Add and Remove that do nothing; or "ref", a table with Is(T) and a Set that does nothing. A class's
-- values are found by its full name, walking up its bases to the first class that has an entry; the build writes one
-- for every class it uses, so that is the class itself. They are kept on the class table.
local valuesKey = {}

local function nothing() end

local function idOf(T)
  return Moonwell.ObjectTypes.IdOf(T)
end

local levelsTable = { __index = {
  get = function (self, level) return self[level] end,
  getCount = function (self) return #self end,
} }

local listTable = { __index = {
  getCount = function (self) return #self end,
  Contains = function (self, T)
    local id = idOf(T)
    for i = 1, #self do
      if self[i] == id then return true end
    end
    return false
  end,
  Add = nothing,
  Remove = nothing,
} }

local refTable = { __index = {
  Is = function (self, T) return self[1] == idOf(T) end,
  Set = nothing,
} }

local function valuesOf(obj)
  local class = getmetatable(obj)
  local values = rawget(class, valuesKey)
  if values ~= nil then return values end
  local found = class
  while found ~= nil and values == nil do
    local name = rawget(found, "__name__")
    if name ~= nil then
      -- A nested class's name has + before its own name; the build names it with a dot.
      values = MoonwellObjectValues[(string.gsub(name, "%+", "."))]
    end
    found = getmetatable(found)
  end
  if values == nil then
    error("Moonwell: the build wrote no object values for " .. tostring(rawget(class, "__name__")))
  end
  rawset(class, valuesKey, values)
  return values
end

function MoonwellField(obj, id, kind)
  local values = valuesOf(obj)
  local value = values[id]
  if kind == "value" then return value end
  if kind == "ref" then
    if type(value) ~= "table" then
      value = setmetatable({ value or 0 }, refTable)
      values[id] = value
    end
    return value
  end
  if value == nil then
    value = {}
    values[id] = value
  end
  if getmetatable(value) == nil then
    setmetatable(value, kind == "levels" and levelsTable or listTable)
  end
  return value
end

-- Where errors happened. The build appends MoonwellLines, a table from the script's lines to C# files and lines (LineTable in
-- Moonwell.Build): files lists the marked files, and map holds triples in script order (the line as the difference from the
-- previous triple's, the file's number or 0 for none, the C# line). CoreSystem calls an exception's traceback where it is
-- thrown and in try's handler, before the stack unwinds; this one reads the stack with error's position, which
-- pcall(error, "", level) gives without the debug library, and keeps the C# locations, innermost first, one a line, at most
-- eight. MoonwellWhere(e) gives them to Errors.Where. A Lua error's message loses its position in war3map.lua.
local lineStarts, lineFiles, lineNumbers

local function loadLines()
  lineStarts, lineFiles, lineNumbers = {}, {}, {}
  local lines = rawget(_G, "MoonwellLines")
  if type(lines) ~= "table" or type(lines.map) ~= "string" then return end
  local line, count, k, triple = 0, 0, 0, {}
  for number in string.gmatch(lines.map, "%d+") do
    k = k + 1
    triple[k] = tonumber(number)
    if k == 3 then
      line = line + triple[1]
      count = count + 1
      lineStarts[count] = line
      lineFiles[count] = lines.files[triple[2]]
      lineNumbers[count] = triple[3]
      k = 0
    end
  end
end

local function locationOf(line)
  if lineStarts == nil then loadLines() end
  local low, high, found = 1, #lineStarts, 0
  while low <= high do
    local middle = (low + high) // 2
    if lineStarts[middle] <= line then
      found = middle
      low = middle + 1
    else
      high = middle - 1
    end
  end
  local file = lineFiles[found]
  if file == nil then return nil end
  return file .. ":" .. lineNumbers[found]
end

local maxLevels = 32
local maxLocations = 8

local function stackLocations()
  local found, count, last = {}, 0, nil
  for level = 2, maxLevels do
    local _, position = pcall(error, "", level)
    local digits = type(position) == "string" and string.match(position, ":(%d+): $")
    local location = digits and locationOf(tonumber(digits))
    if location ~= nil and location ~= last then
      count = count + 1
      found[count] = location
      last = location
      if count == maxLocations then break end
    end
  end
  return table.concat(found, "\n")
end

local function withoutPosition(message)
  if type(message) ~= "string" then return message end
  return string.match(message, '^%[string "war3map%.lua"%]:%d+: (.*)$')
    or string.match(message, "^war3map%.lua:%d+: (.*)$")
    or message
end

-- A rethrown exception keeps where it was first thrown.
System.Exception.traceback = function (this)
  if this.moonwellWhere ~= nil then return end
  this.moonwellWhere = stackLocations()
  this.message = withoutPosition(this.message)
end

function MoonwellWhere(e)
  return e.moonwellWhere or ""
end

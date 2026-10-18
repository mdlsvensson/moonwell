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
  getCount = function (self) return rawget(self, "n") or #self end,
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

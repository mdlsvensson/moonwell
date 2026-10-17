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

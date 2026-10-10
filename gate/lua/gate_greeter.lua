local greeter = {}

function greeter.greet(name)
  return "hello " .. name
end

function greeter.fail()
  error("gate error in a Lua module")
end

return greeter

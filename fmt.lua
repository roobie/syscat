local inspect = require('fennel.fennelview')

local fmt = {}

local metatable = {
  -- Allows `fmt` to be called, and if so works as string.format
  __call = function (self, ...)
    return string.format(...)
  end;
}

setmetatable(fmt, metatable)

function fmt.printf (format, ...)
  print(string.format(format, ...))
end

function fmt.writef (format, ...)
  io.write(string.format(format, ...))
end

function fmt.fwritef (fd, format, ...)
  fd:write(string.format(format, ...))
end

function fmt.inspect (...)
  if select("#", ...) > 1 then
    return inspect({...})
  else
    return inspect(...)
  end
end

function fmt.pp (object)
  print(inspect(object))
end

return fmt

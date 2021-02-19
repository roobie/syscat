--[[
  This file is a pretty ugly hack in order to use local lua_modules.
  See more @ https://scratch.leafo.net/guides/customizing-the-luarocks-tree.html
]]
local version = _VERSION:match("%d+%.%d+")
package.path = 'lua_modules/share/lua/' .. version .. '/?.lua;lua_modules/share/lua/' .. version .. '/?/init.lua;' .. package.path
package.cpath = 'lua_modules/lib/lua/' .. version .. '/?.so;' .. package.cpath

return true

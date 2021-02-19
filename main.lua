
-- Patch the package loader to include the local lua_modules tree
assert(require('set-paths'))


-- Load fennel and the fennel compiler
local fennel = require('fennel.fennel')
table.insert(package.loaders, fennel.make_searcher({correlate=true}))

-- <config
local cfg = require('configuration')
-- >config

local turbo = require('turbo')
local log = require('turbo.log')
local fmt = require('fmt')

local db = require ('db')


local app = turbo.web.Application {
    {"^/$", require('root_handler')};
    {"^/.*%.css$", require('css_handler')};
    {"^/favicon.ico$", require('css_handler')};
}
app.application_name = "syscat-1.0.0"
app:listen(cfg.port)

log.success(fmt("Listening on %d", cfg.port))

-- noreturn
turbo.ioloop.instance():start()

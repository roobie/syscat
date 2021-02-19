(local turbo (require :turbo))
(local turbofennel (require :turbofennel))
(local fmt (require :fmt))

(turbofennel.handler
 :CssHandler
 {:get (fn get [self]
         (self:add_header "Content-Type" "text/css")
         (with-open [f (io.open :static/styles.css :rb)]
           (self:write (f:read :*a))))
  })


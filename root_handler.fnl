(local turbo (require :turbo))
(local turbofennel (require :turbofennel))
(local fmt (require :fmt))
(local h turbofennel.h)
(local layout (require :layout))

(local RootHandler (class "RootHandler" turbo.web.RequestHandler))

(turbofennel.handler
 :RootHandler
 {:get (fn [self]
         (self:write
          (layout.standard
           [(h :h1 {:style "color:red;" :test "{\"test\":1}"} "Testing")
            (h :pre nil [(h :code nil (fmt.inspect self.request.arguments))])
            ])))
  })

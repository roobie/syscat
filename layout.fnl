
(local turbofennel (require :turbofennel))
(local h turbofennel.h)

(local layout {})

(fn layout.standard [contents]
  (.. "<!doctype html>"
      (h :html nil
         [(h :head nil [(h :link {:rel "stylesheet" :type "text/css" :href "styles.css"})])
          (h :body nil
             contents)])))

layout

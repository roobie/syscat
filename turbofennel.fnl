(local turbo (require :turbo))

(local turbofennel {})

(fn turbofennel.handler [name proto]
  (local Handler (class name turbo.web.RequestHandler))
  (each [k v (pairs proto)]
    (tset Handler k v))

  Handler)

(fn stringify-html-attributes [tbl?]
  "Stringify an assoc table into html attributes"
  (fn esc [s]
    (string.gsub (or s "") "\"" "&quot;"))

  (if (= nil tbl?)
      nil
      (let [result {}]
        (each [k v (pairs tbl?)]
          (table.insert result (.. k "=\"" (esc v) "\" ")))
        (table.concat result))))

(fn h-void [tag-name attrs]
  (.. "<" tag-name " " (or (stringify-html-attributes attrs) "") ">\n"))

(fn h-normal [tag-name attrs children]
  (.. "<" tag-name " " (or (stringify-html-attributes attrs) "") ">\n"
      (if (= nil children)
          ""
          (if (= :string (type children))
              children
              (table.concat children)))
      "\n</" tag-name ">\n"))

(fn turbofennel.h [tag-name attrs children]
  "DSL for rendering HTML"
  (local void-tags ",area,base,br,col,command,embed,hr,img,input,keygen,link,menuitem,meta,param,source,track,wbr,")
  (if (not (= nil (string.find void-tags (.. "," tag-name ","))))
      (h-void tag-name attrs)
      (h-normal tag-name attrs children)))

turbofennel

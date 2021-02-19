(local log (require :turbo.log))

(tset package.loaded :_templet (require :turbo-sqlite3._templet))
(tset package.loaded :_xsys (require :turbo-sqlite3._xsys))

(local sqlite (require :turbo-sqlite3.turbo-sqlite3))

(local fmt (require :fmt))

(local cfg (require :configuration))
(local conn (sqlite.open (or cfg.database "")))

(fn pragmas [c]
  (c:exec "PRAGMA synchronous = OFF")
  (c:exec "PRAGMA journal_mode = MEMORY"))
(fn with-transaction [c func]
  (c:exec "BEGIN TRANSACTION")
  (func c)
  (c:exec "END TRANSACTION"))

(local migration-1 {:name "Initial migration"
                    :from-version 0
                    :statements ["
CREATE TABLE categories(
  id INTEGER NOT NULL PRIMARY KEY,
  name TEXT NOT NULL,
  description TEXT,
  inactive INTEGER -- soft delete
)"
                                 "
CREATE TABLE labels(
  id INTEGER NOT NULL PRIMARY KEY,
  name TEXT NOT NULL,
  description TEXT,
  inactive INTEGER -- soft delete
)"
                                 "
CREATE TABLE notes(
  id INTEGER NOT NULL PRIMARY KEY,
  title TEXT NOT NULL,
  body TEXT NOT NULL,
  inactive INTEGER -- soft delete
)"
                                 "
CREATE TABLE m2m_notes_labels(
  note_id INTEGER NOT NULL,
  label_id INTEGER NOT NULL,

  CONSTRAINT fk_note
    FOREIGN KEY (note_id)
    REFERENCES notes(id),

  CONSTRAINT fk_label
    FOREIGN KEY (label_id)
    REFERENCES labels(id)
)"
                                 "
CREATE TABLE m2m_notes_categories(
  note_id INTEGER NOT NULL,
  category_id INTEGER NOT NULL,

  CONSTRAINT fk_note
    FOREIGN KEY (note_id)
    REFERENCES notes(id),

  CONSTRAINT fk_category
    FOREIGN KEY (category_id)
    REFERENCES categories(id)
)"
                                 ]})
(fn apply-migration [c mig]
  (let [result (c:exec "PRAGMA user_version")
        user-version (tonumber (. result.user_version 1))]
    (when (not (= mig.from-version user-version))
      (error (fmt "Wrong user_version(%d) for migration(%s)" user-version mig.name)))
    (with-transaction c (fn []
                          (each [_ stmt (ipairs mig.statements)]
                            (print stmt)
                            (c:exec stmt))
                          (c:exec (fmt "PRAGMA user_version = %d" (+ 1 user-version)))))))

(fn migrate [c]
  (apply-migration c migration-1)
  (let [result (c:exec "PRAGMA user_version")]
    (log.success (fmt "Database user_version = %d" (tonumber (. result.user_version 1))))
    ))

(migrate conn :latest)

(pragmas conn)
(fmt.pp (conn:exec "SELECT name FROM sqlite_master
WHERE type IN ('table','view')
AND name NOT LIKE 'sqlite_%'
ORDER BY 1;"))
(fmt.pp (conn:exec "PRAGMA synchronous"))
(fmt.pp (conn:exec "PRAGMA journal_mode"))
(fmt.pp (conn:exec "PRAGMA foreign_keys"))
;;(fmt.pp (conn:exec "PRAGMA pragma_list"))

{}

# storage

The layouts of the files that something other than the library opens (T-06.1): the SQLite destination, whose table layout is a contract because a reader (a UI or another process, which only reads) depends on it (REQ-OUT-009, Q09-5), and the spool. Identical in Go and Python, versioned. Empty for now.

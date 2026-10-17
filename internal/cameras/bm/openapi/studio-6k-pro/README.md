# Blackmagic Studio Camera 6K Pro: REST API specs

OpenAPI (`swagger/`) and AsyncAPI (`asyncAPI/`) files served by a Studio Camera 6K Pro
at `http://<camera>/control/documentation.html`, saved as-is on 2026-10-17. The three
cameras of the rig serve the same files.

What this firmware does not have:

- `/lens/iris/description`, `/lens/focus/description`, `/lens/zoom/description`. A
  subscription that names them is refused, so the driver subscribes to them in a
  separate request (`lensWatched` in `../../props.go`).

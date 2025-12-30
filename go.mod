module github.com/psantana5/ffmpeg-rtmp

go 1.21

require (
	github.com/google/uuid v1.6.0
	github.com/gorilla/mux v1.8.1
	github.com/mattn/go-sqlite3 v1.14.19
	github.com/psantana5/ffmpeg-rtmp/pkg v0.0.0
	golang.org/x/crypto v0.18.0
)

// Map old pkg paths to new shared/pkg paths
replace github.com/psantana5/ffmpeg-rtmp/pkg => ./shared/pkg

module github.com/psantana5/ffmpeg-rtmp

go 1.21

require (
	github.com/gorilla/mux v1.8.1
	github.com/psantana5/ffmpeg-rtmp/pkg v0.0.0
)

require (
	github.com/google/uuid v1.6.0 // indirect
	github.com/mattn/go-sqlite3 v1.14.19 // indirect
	golang.org/x/crypto v0.18.0 // indirect
)

// Map old pkg paths to new shared/pkg paths
replace github.com/psantana5/ffmpeg-rtmp/pkg => ./shared/pkg

HASH=`git rev-parse HEAD`

prepare:
	rm -f templates.rice-box.go
	rm -f static.rice-box.go
	rice embed-go

check:
	go test -cover -v github.com/nw4869/filebin/app/api github.com/nw4869/filebin/app/model github.com/nw4869/filebin/app/config github.com/nw4869/filebin/app/backend/fs github.com/nw4869/filebin/app/metrics github.com/nw4869/filebin/app/events

get-deps:
	go get github.com/GeertJohan/go.rice
	go get github.com/GeertJohan/go.rice/rice

build: prepare
	go build -ldflags "-X main.githash=${HASH}"

install: prepare
	go install -ldflags "-X main.githash=${HASH}"

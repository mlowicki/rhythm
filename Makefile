all: clean osx linux

osx:
	GOOS=darwin GOARCH=amd64 go build -o dist/rhythm_osx.amd64

linux:
	GOOS=linux GOARCH=amd64 go build -o dist/rhythm_linux.amd64

clean:
	rm -rf dist

runserver:
	go run *.go server -config dev/server.json

.PHONY: all osx linux clean

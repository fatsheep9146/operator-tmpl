.PHONY: build

build:
	GOOS=linux GOARCH=amd64 go build -o operator cmd/main.go

clean:
	rm operator

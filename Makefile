build: ralph-plans

ralph-plans: *.go go.mod go.sum
	go build -o ralph-plans .

install: build
	mkdir -p ~/.local/bin
	cp ralph-plans ~/.local/bin/ralph-plans

.PHONY: build install

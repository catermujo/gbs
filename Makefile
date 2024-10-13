test:
	go test ./...

bench:
	go test -benchmem -run=^$$ -bench . github.com/isinyaaa/gbs

cover:
	go test -coverprofile=./bin/cover.out --cover ./...

clean:
	rm -rf ./bin/*

.PHONY: build demo clean

build:
	go build -o hotreload .

demo: build
	./hotreload --root ./testserver --proxy 8080:8081 --build "go build -o ./bin/testserver ./testserver/main.go" --exec "PORT=8081 ./bin/testserver"

demo-crash: build
	./hotreload --root ./testserver --proxy 8080:8081 --build "go build -o ./bin/testserver ./testserver/main.go" --exec "PORT=8081 ./bin/testserver -crash-mode"

clean:
	rm -rf bin/ hotreload

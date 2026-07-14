.PHONY: run build-web clean upload

run: build-web
	go run ./main.go

build-web:
	cd web && npm install && npm run build

clean:
	rm -rf web/dist

upload: build-web
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o gnas main.go
	scp ./gnas root@192.168.3.112:/tmp/
	ssh root@192.168.3.112 'mv /tmp/gnas /usr/local/bin/ && chmod +x /usr/local/bin/gnas && rc-service gnas restart'

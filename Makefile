.PHONY: run build-web clean upload

DEPLOY_HOST ?= root@192.168.202.129

run: build-web
	go run ./main.go

build-web:
	cd web && npm install && npm run build

clean:
	rm -rf web/dist

upload: build-web
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o gnas main.go
	scp ./gnas $(DEPLOY_HOST):/tmp/gnas
	ssh $(DEPLOY_HOST) 'mv /tmp/gnas /usr/local/bin/gnas && chmod +x /usr/local/bin/gnas && if command -v systemctl >/dev/null 2>&1 && systemctl list-unit-files gnas.service >/dev/null 2>&1; then systemctl restart gnas; elif command -v rc-service >/dev/null 2>&1; then rc-service gnas restart; fi'

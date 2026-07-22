.PHONY: run build-web build-linux build-android clean upload

PUBSPEC_VERSION := $(shell powershell -NoProfile -Command "(Select-String -Path 'gnas_app/pubspec.yaml' -Pattern '^version:\s*([^+\s]+)').Matches[0].Groups[1].Value")
VERSION ?= $(PUBSPEC_VERSION)
TAG ?= v$(VERSION)
RELEASE_DIR ?= release
LINUX_AMD64 := $(RELEASE_DIR)/gnas-linux-amd64
ANDROID_APK := $(RELEASE_DIR)/gnas-android.apk
FLUTTER ?= D:/flutter_develop/flutter/bin/flutter.bat
GH ?= C:/Program Files/GitHub CLI/gh.exe

run: build-web
	go run ./main.go

build-web:
	powershell -NoProfile -ExecutionPolicy Bypass -Command "Set-Location 'web'; npm.cmd ci; if ($$LASTEXITCODE -ne 0) { exit $$LASTEXITCODE }; npm.cmd run build; exit $$LASTEXITCODE"

build-linux: build-web
	powershell -NoProfile -ExecutionPolicy Bypass -Command "New-Item -ItemType Directory -Force '$(RELEASE_DIR)' | Out-Null; $$env:CGO_ENABLED='0'; $$env:GOOS='linux'; $$env:GOARCH='amd64'; go build -trimpath -ldflags '-s -w -X main.version=$(VERSION)' -o '$(LINUX_AMD64)' .; exit $$LASTEXITCODE"

build-android:
	powershell -NoProfile -ExecutionPolicy Bypass -Command "New-Item -ItemType Directory -Force '$(RELEASE_DIR)' | Out-Null; Set-Location 'gnas_app'; & '$(FLUTTER)' pub get; if ($$LASTEXITCODE -ne 0) { exit $$LASTEXITCODE }; & '$(FLUTTER)' build apk --release; if ($$LASTEXITCODE -ne 0) { exit $$LASTEXITCODE }; Copy-Item 'build/app/outputs/flutter-apk/app-release.apk' '../$(ANDROID_APK)' -Force"

clean:
	powershell -NoProfile -ExecutionPolicy Bypass -Command "Remove-Item -Recurse -Force 'web/dist','$(RELEASE_DIR)' -ErrorAction SilentlyContinue"

upload:
	$(MAKE) build-linux
	$(MAKE) build-android
	powershell -NoProfile -ExecutionPolicy Bypass -Command "& '$(GH)' auth status; if ($$LASTEXITCODE -ne 0) { exit $$LASTEXITCODE }; & '$(GH)' release view '$(TAG)' *> $$null; if ($$LASTEXITCODE -eq 0) { & '$(GH)' release upload '$(TAG)' '$(LINUX_AMD64)' '$(ANDROID_APK)' --clobber } else { & '$(GH)' release create '$(TAG)' '$(LINUX_AMD64)' '$(ANDROID_APK)' --target main --title 'GNAS $(TAG)' --generate-notes }; exit $$LASTEXITCODE"

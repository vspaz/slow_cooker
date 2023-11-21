TARGET=slow_cooker
TARGET_BUILD_REVISION=`git rev-parse --short HEAD`
LDFLAGS="-s -w"

all: build
build:
	go build -ldflags=$(LDFLAGS) -o $(TARGET) main.go; upx $(TARGET)

.PHONY: test
test:
	go test -race -v

.PHONY: clean
clean:
	rm -f $(TARGET)

.PHONY: style-fix
style-fix:
	gofmt -w .
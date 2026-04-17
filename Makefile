.PHONY: build clean

build:
	go build -o out/mihomo-cli .
	cp config.json out/
	cp -r service.d out/
	cp base.yml out/
	cp override.yml out/
	cp README.md out/

clean:
	rm -rf out

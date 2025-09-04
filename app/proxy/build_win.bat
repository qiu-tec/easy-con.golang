go build -ldflags="-s -w --extldflags '-static -fpic'" -o ./output/QFProxy.exe
upx --best ./output/QFProxy.exe

MACICONS := darwin/Contents/Resources/ftpsync.icns
WIX := "C:\Program Files (x86)\WIX Toolset v3.11\bin"
GO_FILES := main.go config.go cache.go help.go ftp.go parser.go scanner.go report.go viewer.go
RESOURCES := $(wildcard images/*.png) help.html resources.qrc

ifeq ($(OS),Windows_NT)
	PLATFORM := windows
	EXE := deploy/windows/ftpsync.exe
else
	PLATFORM := $(shell uname | tr A-Z a-z)
	ifeq ($(PLATFORM),darwin)
		EXE := deploy/darwin/ftpsync.app/Contents/MacOS/ftpsync
	else
		EXE := deploy/$(PLATFORM)/ftpsync
	endif
endif

default: $(PLATFORM)

darwin: $(MACICONS) $(EXE)

windows: ftpsync.syso $(EXE) ftpsync.msi

$(EXE): $(GO_FILES) $(RESOURCES)
	qtdeploy build

$(MACICONS): ftpsync.iconset/*
	iconutil --convert icns ftpsync.iconset

ftpsync.syso: ftpsync.exe.manifest ftpsync.ico
	rsrc -o $@ -arch amd64 -manifest ftpsync.exe.manifest -ico ftpsync.ico

ftpsync.msi: ftpsync.wxs $(EXE)
	$(WIX)\candle -arch x64 ftpsync.wxs
	$(WIX)\light -ext WixUIExtension ftpsync.wixobj

clean:
	rm -f moc.go moc.cpp moc.h moc_moc.h moc_cgo_*.go
	rm -f rcc.cpp rcc_cgo_*.go
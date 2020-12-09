#
# Assumes GNU Make and on Windows assumes standard 'cmd.exe' shell

MACICONS := darwin/Contents/Resources/ftpsync.icns
PLIST := darwin/Contents/info.plist
WIX := "C:\Program Files (x86)\WIX Toolset v3.11\bin"
GO_FILES := main.go $(wildcard app/*.go)
RESOURCES := $(wildcard res/images/*.png) res/help.html resources.qrc
CLEAN := $(strip $(wildcard moc*.*) $(wildcard rcc*.*) $(wildcard ftpsync.wix*))

ifeq ($(OS),Windows_NT)
	PLATFORM := windows
	EXE := deploy/windows/ftpsync.exe
	RM := del
else
	PLATFORM := $(shell uname | tr A-Z a-z)
	RM := rm
	ifeq ($(PLATFORM),darwin)
		EXE := deploy/darwin/ftpsync.app/Contents/MacOS/ftpsync
	else
		EXE := deploy/$(PLATFORM)/ftpsync
	endif
endif

default: $(PLATFORM)

darwin: $(MACICONS) $(PLIST) $(EXE)
	rm -f darwin/Contents/Resources/Qml/*

windows: ftpsync.syso $(EXE) ftpsync.msi

$(EXE): $(GO_FILES) $(RESOURCES)
	qtdeploy build desktop

$(MACICONS): res/ftpsync.iconset/*
	mkdir -p darwin/Contents/Resources
	iconutil --convert icns res/ftpsync.iconset -o $@

$(PLIST): res/info.plist
	cp $< $@

ftpsync.syso: res/ftpsync.exe.manifest res/ftpsync.ico
	rsrc -o $@ -arch amd64 -manifest res/ftpsync.exe.manifest -ico res/ftpsync.ico

ftpsync.msi: res/ftpsync.wxs $(EXE)
	$(WIX)\candle -arch x64 res/ftpsync.wxs
	$(WIX)\light -ext WixUIExtension ftpsync.wixobj

clean: $(CLEAN)
ifneq ($(CLEAN),)
	$(RM) $(CLEAN)
endif
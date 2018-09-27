PKG := github.com/zchee/ghctl
BIN := ghctl
GO_SRCS = $(shell find . -type f \( -name '*.go' -and -not -iwholename '*testdata' \) )

GO_BUILD_FLAGS = -v -x 
ifneq ($(RACE),)
GO_BUILD_FLAGS += -race
endif

bulid: ghctl

ghctl: $(GO_SRCS)
	go build -o $@ $(GO_BUILD_FLAGS) .

install: $(GO_SRCS)
	go install $(GO_BUILD_FLAGS) .

clean:
	rm -rf ${BIN} *.out

.PHONY: build install clean

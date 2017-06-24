GO_SRCS = $(shell find . -type f \( -name '*.go' -and -not -iwholename '*testdata' \) )

GO_BUILD_FLAGS = -v -x 
ifneq ($(GHCTL_DEBUG),)
GO_BUILD_FLAGS += -race
endif

bulid: bin/ghctl

bin/ghctl: $(GO_SRCS)
	go build -o $@ $(GO_BUILD_FLAGS) .

install: $(GO_SRCS)
	go install $(GO_BUILD_FLAGS) .

clean:
	rm -rf bin

.PHONY: build install clean

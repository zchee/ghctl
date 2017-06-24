GO_BUILD_FLAGS = -v -x 
ifneq ($(GHCTL_DEBUG),)
GO_BUILD_FLAGS += -race
endif

bulid: bin/ghctl

bin/ghctl:
	go build -o $@ $(GO_BUILD_FLAGS) .

install:
	go install $(GO_BUILD_FLAGS) .

clean:
	rm -rf bin

.PHONY: build install clean

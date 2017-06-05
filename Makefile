ifneq ($(GHCTL_DEBUG),)
default: install/race
else
default: install
endif

install:
	go install -v -x .

install/race:
	go install -v -x -race .

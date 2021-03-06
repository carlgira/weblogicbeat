BEAT_NAME=weblogicbeat
BEAT_PATH=github.com/carlgira/weblogicbeat
BEAT_GOPATH=$(firstword $(subst :, ,${GOPATH}))
SYSTEM_TESTS=false
TEST_ENVIRONMENT=false
ES_BEATS?=./vendor/github.com/elastic/beats
GOPACKAGES=$(shell govendor list -no-status +local)
GOBUILD_FLAGS=-i -ldflags "-X $(BEAT_PATH)/vendor/github.com/elastic/beats/libbeat/version.buildTime=$(NOW) -X $(BEAT_PATH)/vendor/github.com/elastic/beats/libbeat/version.commit=$(COMMIT_ID)"
MAGE_IMPORT_PATH=${BEAT_PATH}/vendor/github.com/magefile/mage

# Path to the libbeat Makefile
-include $(ES_BEATS)/libbeat/scripts/Makefile

# Initial beat setup
.PHONY: setup
setup: copy-vendor git-init update git-add

# Copy beats into vendor directory
.PHONY: copy-vendor
copy-vendor:
	mkdir -p vendor/github.com/elastic
	cp -R ${BEAT_GOPATH}/src/github.com/elastic/beats vendor/github.com/elastic/
	rm -rf vendor/github.com/elastic/beats/.git vendor/github.com/elastic/beats/x-pack
	mkdir -p vendor/github.com/magefile
	cp -R ${BEAT_GOPATH}/src/github.com/elastic/beats/vendor/github.com/magefile/mage vendor/github.com/magefile
	cp -r ${BEAT_GOPATH}/src/github.com/Jeffail vendor/github.com
	rm -rf ${BEAT_GOPATH}/src/github.com/Jeffail/gabs/.git
	cp -rf ${BEAT_GOPATH}/src/github.com/Jeffail vendor/github.com
	rm -rf ${BEAT_GOPATH}/src/github.com/Jeffail/gabs/.git
	cp -rf ${BEAT_GOPATH}/src/gopkg.in vendor/gopkg.in
	rm -rf ${BEAT_GOPATH}/src/gopkg.in/resty.v1/.git
	cp -rf ${BEAT_GOPATH}/src/golang.org vendor/golang.org

.PHONY: git-init
git-init:
	git init

.PHONY: git-add
git-add:
	git add -A
	git commit -m "Add generated weblogicbeat files"

# Collects all dependencies and then calls update
.PHONY: collect
collect:

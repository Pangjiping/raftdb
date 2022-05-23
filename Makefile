GOFMT_FILES?=$$(find . -name '*.go')
NODE_ID=node-$$(ifconfig -a|grep inet|grep -v 127.0.0.1|grep -v inet6|awk '{print $2}'|tr -d "addr:" | md5)
RAFT_ADDRESS = localhost:8089
HTTP_ADDRESS = localhost:8091
LEADER_ADDRESS = leader:8091 # 能不能从配置文件获取？


# unit testing for raftDB
test:
	rm -f ./*.out
	go test ./pkg/httpd -v -run=Test -timeout=5m -cover -coverprofile raftDB_test_httpd.out
	go test ./pkg/store -v -run=Test -timeout=5m -cover -coverprofile raftDB_test_store.out

# compile raftDB for macos or linux
fmt:
	gofmt -w $(GOFMT_FILES)
	goimports -w $(GOFMT_FILES)

clean:
	rm -rf ./bin/*

mac_build:
	GOOS=darwin GOARCH=amd64 go build ./cmd/raftd
	cp ./raftd ./bin/raftdb
	rm -f ./raftd

linux_build:
	GOOS=linux GOARCH=amd64 go build ./cmd/raftd
	cd ./raftd ./bin/raftdb
	rm -f ./raftd

mac_dev: clean fmt mac_build
linux_dev: clean fmt linux_build

# run raftDB
run:
	./bin/raftdb

# raft server
start:
	./bin/raftdb -id $(NODE_ID) -haddr $(HTTP_ADDRESS) -raddr $(RAFT_ADDRESS) ~/.raftdb

join:
	./bin/raftdb -id $(NODE_ID) -haddr $(HTTP_ADDRESS) -raddr $(RAFT_ADDRESS) -join $(LEADER_ADDRESS) ~/.raftdb

# key-value
set:
	curl -XPOST localhost:8091/key -d '{"foo":"bar"}' -L

get:
	curl -XGET localhost:8091/key/foo?level=consistent -L

delete:
	curl -XDELETE localhost:8091/key/foo

.PHONY: test mac_dev linux_dev run start join set get delete
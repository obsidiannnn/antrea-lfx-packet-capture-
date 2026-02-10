.PHONY: build deploy clean test

build:
	go build ./...

deploy:
	kubectl apply -f config/crd/
	kubectl apply -f config/deploy/

clean:
	kubectl delete -f config/deploy/ || true
	kubectl delete -f config/crd/ || true

test:
	go test ./...

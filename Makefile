
template_templ.go: template.templ
	templ generate

fand: *.go template_templ.go
	go build -o fand .

.PHONY: deploy
deploy: fand
	scp fand 10.233.0.1:


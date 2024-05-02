PHONY: cover1 coverast

# cover1 runs 'go test -cover' on each package in isolation, so that coverage for
# this package's unit tests are shown. Running go test ./... -cover doesn't work
# in this case, for example the scanner package gets those results:
#   go test ./... -cover
#     ok  	github.com/mna/nenuphar/lang/scanner	(cached)	coverage: 50.5% of statements
#   go test ./lang/scanner -cover
#     ok  	github.com/mna/nenuphar/lang/scanner	(cached)	coverage: 97.1% of statements
cover1:
	go list ./... | xargs -n 1 go test -cover

# coverast tests the parser package but targets the AST package for coverage,
# as this package is tested via the parser. This gives the real test coverage
# for the ast package.
coverast:
	go test ./lang/parser -coverpkg ./lang/ast


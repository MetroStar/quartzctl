# Quartz CLI contributor guidance

Follow [CONTRIBUTING.md](CONTRIBUTING.md). Use existing Go configuration,
provider and lifecycle helpers; preserve native Linux, macOS and Windows
behavior and fail explicitly on invalid operator input.

Run `go test` on changed packages, `go vet` on those packages, and format changed
Go files with `gofmt`. Run the repository lint/build checks before publishing.
Do not treat cross-compilation as native runtime or live AWS validation.

Changes go through reviewed pull requests targeting `main`; no direct main
pushes, unapproved merges or changes to shared resources during acceptance.

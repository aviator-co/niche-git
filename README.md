# niche-git

niche-git is a niche git utility.

## Motivation

Git protocol has been evolved to support more advanced operations. However, in
order to harness that capability, C-git client is not enough in some cases. This
project provides a utility for such niche use cases.

## How to use it

### Getting modified files

```bash
go run cmd/niche-git/main.go get-modified-files \
    --repo-url https://github.com/git/git \
    --commit-hash1 3c2a3fdc388747b9eaf4a4a4f2035c1c9ddb26d0 \
    --commit-hash2 efb050becb6bc703f76382e1f1b6273100e6ace3
```

With authn:

```bash
go run cmd/niche-git/main.go get-modified-files \
    --repo-url https://github.com/draftcode/some-private-repo \
    --commit-hash1 998122b45e63b2999d57a1af9e74761c0524e932 \
    --commit-hash2 f27a58920f7319cc7b62e55cf3095d1ee2ab1dde \
    --basic-authz-user x-acess-token \
    --basic-authz-password "$(gh auth token)"
```

## Adding a license header

```bash
addlicense -c "Aviator Technologies, Inc." -l mit -s=only .
```

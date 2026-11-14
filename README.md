# gofub

Find universal binaries in Go.

gofub scans macOS app bundles and groups their main executables by the CPU
architectures they're built for:

- **Intel**: x86_64 only
- **Apple**: arm64 only (Apple silicon)
- **Universal**: both arm64 and x86_64
- **Other/Unknown**: anything else, such as a shell-script launcher

It's handy for spotting apps that still need Rosetta 2. gofub reads the Mach-O
headers directly, so it needs no external tools. It only works on macOS.

## Install

Clone the repository and build it:

```sh
git clone https://github.com/mikemadden42/gofub.git
cd gofub
go build
```

## Usage

```
gofub [-r] [dir ...]
```

With no directories, gofub scans `/Applications`. Pass `-r` to also search
subfolders such as `/Applications/Utilities`. Bundles inside other bundles,
like helper apps, are skipped.

```sh
gofub                                   # top level of /Applications
gofub -r                                # /Applications and its subfolders
gofub -r /Applications ~/Applications   # several directories
```

Results go to stdout. Warnings about bundles that can't be read go to stderr.
gofub exits with status 1 if a directory you name can't be read.

Example output (abridged):

```
Intel Binaries
/Applications/Legacy Tool.app/Contents/MacOS/Legacy Tool

Apple Binaries
/Applications/Xcode.app/Contents/MacOS/Xcode

Universal Binaries
/Applications/Pages.app/Contents/MacOS/Pages
/Applications/Safari.app/Contents/MacOS/Safari

Other/Unknown
```

## Development

```sh
go test ./...   # run tests
make check      # golangci-lint
make build      # snapshot build with goreleaser, including a universal binary
```

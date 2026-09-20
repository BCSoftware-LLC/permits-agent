# Third-party notices

The initial Go candidate used cli-printing-press 4.31.1 templates, released at `a6d572b02ed02175df95a8df7a278f970107513d`. Handwritten native data, reference, CLI and MCP changes are maintained by BC Software. The prior Python application was handwritten, not generator output. Generated provenance does not establish correctness; the native release is independently tested.

## cli-printing-press

Source: https://github.com/mvanhorn/cli-printing-press

MIT License

Copyright (c) 2026 Matt Van Horn and Trevin Chow

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.

## Runtime dependencies

Go dependency versions and checksums are pinned in `go.mod` / `go.sum`. Source distributions and license texts are retrievable via `go mod download`; Go binaries contain their module inventory (`go version -m <binary>`). BC Software’s proprietary license applies only to its own code, not these separately licensed dependencies. SQLite is public domain.

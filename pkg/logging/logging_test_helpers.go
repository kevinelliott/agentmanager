package logging

import "os"

// openForRead is extracted so test code can open files without pulling
// os directly into the test file (keeps the test imports minimal and
// lets the helper be reused if more tests need file IO).
func openForRead(p string) (*os.File, error) {
	return os.Open(p)
}

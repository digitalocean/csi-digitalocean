package driver

type testAttachmentValidator struct {
	readFileFunc     func(name string) ([]byte, error)
	evalSymlinksFunc func(path string) (string, error)
}

func (av *testAttachmentValidator) readFile(name string) ([]byte, error) {
	return av.readFileFunc(name)
}

func (av *testAttachmentValidator) evalSymlinks(path string) (string, error) {
	return av.evalSymlinksFunc(path)
}

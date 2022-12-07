package test

// T our minimal testing interface for our custom assertions
type T interface {
	Log(args ...interface{})
	Logf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
	FailNow()
	Fail()
	Fatalf(format string, args ...interface{})
}

func NewMockT() *MockT {
	return &MockT{}
}

var _ T = &MockT{}

type MockT struct {
	logfCount    int
	errorfCount  int
	fatalfCount  int
	failnowCount int
	failCount    int
}

func (t *MockT) Log(args ...interface{}) {
	t.logfCount++
}

func (t *MockT) Logf(format string, args ...interface{}) {
	t.logfCount++
}

func (t *MockT) Errorf(format string, args ...interface{}) {
	t.errorfCount++
}

func (t *MockT) Fatalf(format string, args ...interface{}) {
	t.fatalfCount++
}

func (t *MockT) FailNow() {
	t.failnowCount++
}

func (t *MockT) Fail() {
	t.failCount++
}

func (t *MockT) CalledLogf() bool {
	return t.logfCount > 0
}

func (t *MockT) CalledErrorf() bool {
	return t.errorfCount > 0
}

func (t *MockT) CalledFatalf() bool {
	return t.fatalfCount > 0
}

func (t *MockT) CalledFailNow() bool {
	return t.failnowCount > 0
}

package logging

import "fmt"
import "time"

import klog "k8s.io/klog/v2"

var TRACE int = 0
var DEBUG = 1
var INFO = 2
var WARNING = 3
var ERROR = 4
var FATAL = 5

var Logger = logger{}

type logger struct {
	Level    int  // 0 = trace, 1 = debug, 2 = info, 3 = warning, 4 = error, 5 = fatal
	FailFast bool // Should even errors be fatal?
}

func (l *logger) Trace(msg string, params ...interface{}) {
	if l.Level <= TRACE {
		klog.Infof("TRACE "+msg, params...)
	}
}

func (l *logger) Debug(msg string, params ...interface{}) {
	if l.Level <= DEBUG {
		klog.Infof("DEBUG "+msg, params...)
	}
}

func (l *logger) Info(msg string, params ...interface{}) {
	if l.Level <= INFO {
		klog.Infof("INFO "+msg, params...)
	}
}

func (l *logger) Warning(msg string, params ...interface{}) {
	if l.Level <= WARNING {
		klog.Infof("WARNING "+msg, params...)
	}
}

func (l *logger) Error(msg string, params ...interface{}) {
	if l.Level <= ERROR {
		if l.FailFast {
			klog.Fatalf("ERROR(fatal)"+msg, params...)
		} else {
			klog.Errorf("ERROR "+msg, params...)
		}
	}
}

func (l *logger) Fatal(msg string, params ...interface{}) {
	if l.Level <= FATAL {
		klog.Fatalf("FATAL "+msg, params...)
	}
}

// Log test failure with error code so we can compile a statistic later
func (l *logger) Fail(errCode int, msg string, params ...interface{}) error {
	errorMessage := fmt.Sprintf("FAIL(%d): %s", errCode, msg)
	klog.Infof(errorMessage, params...)
	data := ErrorEntry{
		Timestamp: time.Now(),
		Code:      errCode,
		Message:   fmt.Sprintf(errorMessage, params...),
	}
	errorsQueue <- data
	return fmt.Errorf(errorMessage, params...)
}

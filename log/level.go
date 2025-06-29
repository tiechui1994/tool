package log

import "github.com/sirupsen/logrus"

type Level = logrus.Level

const (
	TraceLevel Level = logrus.TraceLevel
	DebugLevel Level = logrus.DebugLevel
	InfoLevel Level = logrus.InfoLevel
	WarnLevel Level = logrus.WarnLevel
	ErrorLevel Level = logrus.ErrorLevel
	FatalLevel Level = logrus.FatalLevel
	PanicLevel Level = logrus.PanicLevel
)
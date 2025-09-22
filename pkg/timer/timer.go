package timer

import (
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
)

type Timer interface {
	Track(start time.Time, funcName string)
}

type TimerImpl struct {
	*logrus.Logger
}

func NewTimerImpl(logger *logrus.Logger) (*TimerImpl, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger is nil")
	}
	return &TimerImpl{
		Logger: logger,
	}, nil
}

func (t *TimerImpl) Track(start time.Time, name string) {
	elapsed := time.Since(start)
	t.Infof("%s cost %s", name, elapsed.Truncate(time.Millisecond).String())
}

package log

import "codeberg.org/crowci/crow/v5/server/model"

type Service interface {
	LogFind(step *model.Step) ([]*model.LogEntry, error)
	LogAppend(step *model.Step, logEntries []*model.LogEntry) error
	LogDelete(step *model.Step) error
}

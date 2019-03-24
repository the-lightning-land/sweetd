package pairing

import "github.com/the-lightning-land/sweetd/ap"

type Config struct {
	Logger      Logger
	AdapterId   string
	AccessPoint ap.Ap
}
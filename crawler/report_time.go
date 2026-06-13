package crawler

import "time"

var reportTime = func() time.Time {
	return time.Now().UTC()
}

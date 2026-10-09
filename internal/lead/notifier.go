package lead

import "context"

type Notifier interface {
	Notify(context.Context, Lead) error
}

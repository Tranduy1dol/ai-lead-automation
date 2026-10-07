package lead

import "context"

type Classifier interface {
	Classify(context.Context, Lead) (Classification, error)
}

type ClassifierFunc func(context.Context, Lead) (Classification, error)

func (f ClassifierFunc) Classify(ctx context.Context, lead Lead) (Classification, error) {
	return f(ctx, lead)
}

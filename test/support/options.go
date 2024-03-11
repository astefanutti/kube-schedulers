package support

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type Option[T any] interface {
	applyTo(to T) error
}

func applyOptions[T any](to T, options ...Option[T]) error {
	for _, option := range options {
		if err := option.applyTo(to); err != nil {
			return err
		}
	}
	return nil
}

type errorOption[T any] func(to T) error

var _ Option[any] = errorOption[any](nil)

// nolint: unused
// To be removed when the false-positivity is fixed.
func (o errorOption[T]) applyTo(to T) error {
	return o(to)
}

type listOption struct {
	labelSelector string
}

func LabelSelector(selector string) Option[*metav1.ListOptions] {
	return &listOption{
		labelSelector: selector,
	}
}

var _ Option[*metav1.ListOptions] = (*listOption)(nil)

func (o *listOption) applyTo(options *metav1.ListOptions) error {
	options.LabelSelector = o.labelSelector
	return nil
}

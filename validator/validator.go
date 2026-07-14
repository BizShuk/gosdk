// Package validator provides a composable validation framework.
//
// The package exposes a single contract — IValidator.Validate() error — that
// any concrete validator must satisfy. Validators can be aggregated
// recursively via the composite Validator so a tree of validation rules is
// evaluated in one call.
package validator

// IValidator is the contract for any validator. Implementations must be safe
// to call repeatedly and must return nil on success.
//
// The prefix "I" is preserved here as an explicit marker that the type is an
// interface, matching the terminology used at the call site. The rest of the
// codebase uses idiomatic Go naming (Reader, Writer, Notifier, Service) and
// does not rely on the "I" prefix.
type IValidator interface {
	// Validate evaluates the validator and returns nil on success.
	// It must not retain or mutate any state held by the caller.
	Validate() error
}

// Validator composes multiple IValidator instances into a single one. The
// composite itself implements IValidator, so it can be nested arbitrarily.
//
// Validate stops at the first error and returns it. This is fail-fast and
// matches the typical usage of validating request payloads before they are
// handed to a handler.
type Validator struct {
	validators []IValidator
}

// Compile-time assertion that *Validator satisfies IValidator. If the
// signature of Validate drifts, the package will fail to build here instead
// of at a distant call site.
var _ IValidator = (*Validator)(nil)

// New returns a composite Validator that runs the supplied validators in
// the order they were passed. Order matters: validation stops at the first
// non-nil error and returns it.
//
// The input slice is defensively copied so mutations made by the caller
// after construction do not affect the composite.
func New(validators ...IValidator) *Validator {
	cp := make([]IValidator, len(validators))
	copy(cp, validators)
	return &Validator{validators: cp}
}

// Add appends one or more validators to the composite and returns the
// receiver so calls can be chained:
//
//	v := validator.New().
//	    Add(string.NewNotEmpty(name)).
//	    Add(string.NewMinLen(name, 3))
func (v *Validator) Add(validators ...IValidator) *Validator {
	v.validators = append(v.validators, validators...)
	return v
}

// Validate walks the contained validators in registration order and returns
// the first error encountered. A nil error means every validator passed.
//
// Because *Validator itself implements IValidator, a child Validator may
// contain other Validators recursively. Recursion terminates at the first
// failing leaf.
func (v *Validator) Validate() error {
	for _, val := range v.validators {
		if err := val.Validate(); err != nil {
			return err
		}
	}
	return nil
}

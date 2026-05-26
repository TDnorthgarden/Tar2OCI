package errors

import "fmt"

// Error codes as defined in the PRD
const (
	E001 = "E001" // input file not found
	E002 = "E002" // invalid tar format
	E003 = "E003" // permission denied
	E004 = "E004" // auth failed
	E005 = "E005" // missing image repository
	E006 = "E006" // missing input files
	E007 = "E007" // invalid platform format
	E008 = "E008" // base image not found
	E009 = "E009" // disk space insufficient
	E010 = "E010" // network timeout
	E011 = "E011" // registry API error
	E012 = "E012" // unsupported compression
	E013 = "E013" // config file parse error
	E014 = "E014" // conflicting parameters
	E015 = "E015" // layer digest mismatch
)

// Tar2OCIError represents a Tar2OCI error with code
type Tar2OCIError struct {
	Code    string
	Message string
	Err     error
}

func (e *Tar2OCIError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *Tar2OCIError) Unwrap() error {
	return e.Err
}

// New creates a new Tar2OCI error
func New(code, message string) *Tar2OCIError {
	return &Tar2OCIError{
		Code:    code,
		Message: message,
	}
}

// Wrap wraps an existing error with a code
func Wrap(code string, err error) *Tar2OCIError {
	return &Tar2OCIError{
		Code:    code,
		Message: err.Error(),
		Err:     err,
	}
}

// Wrapf wraps an existing error with a code and formatted message
func Wrapf(code, format string, args ...interface{}) *Tar2OCIError {
	return &Tar2OCIError{
		Code:    code,
		Message: fmt.Sprintf(format, args...),
	}
}

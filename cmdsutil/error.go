package cmdsutil

// ErrorType signfies a category of errors
type ErrorType uint

// ErrorTypes convey what category of error ocurred
const (
	ErrNormal         ErrorType = iota // general errors
	ErrClient                          // error was caused by the client, (e.g. invalid CLI usage)
	ErrImplementation                  // programmer error in the server
	// TODO: add more types of errors for better error-specific handling
)

// Error is a struct for marshalling errors
type Error struct {
	Message string
	Code    ErrorType
}

func (e Error) Error() string {
	return e.Message
}

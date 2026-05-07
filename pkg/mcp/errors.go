package mcp

// ParamError signals that a tool-call argument was missing or invalid.
// handleToolsCall maps this error type to JSON-RPC code -32602
// (InvalidParams), distinguishing user-attributable validation failures
// from -32603 (InternalError) which is reserved for filesystem/marshaling
// failures and other unexpected internals.
//
// Use NewParamError to construct one. Use errors.As to detect, so wrapping
// via fmt.Errorf("...: %w", paramErr) still surfaces the right code.
type ParamError struct {
	Msg string
}

func (e *ParamError) Error() string { return e.Msg }

// NewParamError returns an error of type *ParamError.
func NewParamError(msg string) *ParamError { return &ParamError{Msg: msg} }

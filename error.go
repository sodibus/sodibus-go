package sodigo

import "fmt"

const (
	ERR_FrameUnsynchronized = 1
	// >= 1000 will be protocol error
)

type SdError struct {
	Code int
}

func (e SdError) Error() string {
	return fmt.Sprintf("SODIBus Error: %p", e.Code)
}

func NewSdError(code int) SdError {
	return SdError{ Code: code }
}

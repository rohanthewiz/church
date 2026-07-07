package basectlr

import (
	"fmt"

	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/serr"
)

// recoverMsg is what the visitor sees when a page render panics in production
// (non-prod environments skip recovery so the panic surfaces during dev).
const recoverMsg = "Oops, we encountered a server error. Try refreshing the page."

func logPanic(p interface{}) {
	logger.LogErr(serr.NewSErr("Panic occurred", "panic", fmt.Sprintf("%v", p)))
}

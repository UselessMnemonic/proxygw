package defaults

import (
	"proxygw/internal/frontend"
	"proxygw/internal/target"
)

func PreloadTargetKinds() map[string]target.Kind {
	result := make(map[string]target.Kind)
	result[NilTarget{}.Name()] = NilTarget{}
	return result
}

func PreloadFrontendKinds() map[string]frontend.Kind {
	result := make(map[string]frontend.Kind)
	result[NilFrontend{}.Name()] = NilFrontend{}
	return result
}

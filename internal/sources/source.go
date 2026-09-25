package sources

import (
	"context"

	"github.com/adriankarlen/yeet/internal/model"
)

type Source interface {
	Name() string
	List(context.Context) (model.Sessions, error)
}

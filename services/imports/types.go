package imports

type Service interface {
	Import() error
}

type Impl struct{}

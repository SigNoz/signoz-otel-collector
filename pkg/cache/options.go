package cache

type options struct {
	host     string
	port     int
	user     string
	password string
	database int
}

type Option func(*options)

func WithHost(s string) Option {
	return func(o *options) {
		o.host = s
	}
}

func WithPort(i int) Option {
	return func(o *options) {
		o.port = i
	}
}

func WithUser(s string) Option {
	return func(o *options) {
		o.user = s
	}
}

func WithPassword(s string) Option {
	return func(o *options) {
		o.password = s
	}
}

func WithDatabase(i int) Option {
	return func(o *options) {
		o.database = i
	}
}

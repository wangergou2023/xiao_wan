package vector

// Option provides a function definition to set options
type Option func(*options)

// options holds the options for the vector robot.
type options struct {
	SerialNo  string
	RobotName string `ini:"name"`
	CertPath  string `ini:"cert"`
	Target    string `ini:"ip"`
	Token     string `ini:"guid"`
}

// WithTarget sets the ip of the vector robot.
func WithTarget(s string) Option {
	return func(o *options) {
		if len(s) > 0 {
			o.Target = s
		}
	}
}

// WithToken set the token for the vector robot.
func WithToken(s string) Option {
	return func(o *options) {
		if len(s) > 0 {
			o.Token = s
		}
	}
}

// WithSerialNo set the serialno for the vector robot.
func WithSerialNo(s string) Option {
	return func(o *options) {
		if len(s) > 0 {
			o.SerialNo = s
		}
	}
}

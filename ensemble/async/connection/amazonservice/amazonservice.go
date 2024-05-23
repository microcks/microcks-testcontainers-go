package amazonservice

// Connection represents an Amazon Service connection settings.
type Connection struct {
	// Region represents a region
	Region string

	// EndpointOverride represents an endpoint override
	EndpointOverride string

	// AccessKey represents an access key
	accessKey string

	// SccessKey represents a secret key
	SecretKey string
}

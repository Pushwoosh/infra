package mongo

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

type ConnectionReadPreference string

const (
	ReadPreferencePrimary            ConnectionReadPreference = "primary"
	ReadPreferencePrimaryPreferred   ConnectionReadPreference = "primary preferred"
	ReadPreferenceSecondaryPreferred ConnectionReadPreference = "secondary preferred"
	ReadPreferenceSecondary          ConnectionReadPreference = "secondary"
	ReadPreferenceNearest            ConnectionReadPreference = "nearest"
)

// Container is a simple container for holding named mongo connections.
// It's a helper for sharing same connection between several components.
type Container struct {
	mu   *sync.RWMutex
	cfg  map[string]ConnectionConfig
	pool map[string]*mongo.Client
}

func NewContainer() *Container {
	return &Container{
		mu:   &sync.RWMutex{},
		cfg:  make(map[string]ConnectionConfig),
		pool: make(map[string]*mongo.Client),
	}
}

// Connect creates a new named mongo connection.
func (cont *Container) Connect(appName string, name string, cfg *ConnectionConfig) error {
	opts := options.Client()
	opts.ApplyURI(cfg.URI)
	opts.SetAppName(appName)

	addresses, err := prepareAddresses(opts.Hosts)
	if err != nil {
		return errors.Wrap(err, "prepareAddresses")
	}

	opts.SetHosts(addresses)

	timeout := time.Second * 10
	if opts.ConnectTimeout != nil {
		timeout = *opts.ConnectTimeout
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	mb := &monitorBuilder{}
	initMetricsMonitor(mb, appName)
	initLoggingMonitor(mb, cfg.QueryLog)

	opts.SetMonitor(mb.Build())

	// connect and ping
	client, err := mongo.Connect(ctx, opts)
	if err != nil {
		return err
	}

	err = client.Ping(ctx, opts.ReadPreference)
	if err != nil {
		return errors.Wrap(err, "ping")
	}

	cont.mu.Lock()
	defer cont.mu.Unlock()

	cont.pool[name] = client
	cont.cfg[name] = *cfg

	return nil
}

// Get gets a connection from the container
func (cont *Container) Get(name string) *mongo.Client {
	cont.mu.RLock()
	defer cont.mu.RUnlock()

	return cont.pool[name]
}

func ReadPreferenceFromString(src ConnectionReadPreference) *readpref.ReadPref {
	switch src {
	case ReadPreferencePrimary:
		return readpref.Primary()
	case ReadPreferencePrimaryPreferred:
		return readpref.PrimaryPreferred()
	case ReadPreferenceSecondaryPreferred:
		return readpref.SecondaryPreferred()
	case ReadPreferenceSecondary:
		return readpref.Secondary()
	case ReadPreferenceNearest:
		return readpref.Nearest()
	default:
		return nil
	}
}

// prepareAddresses
// We use DNS to group several mongos's together, but the mongo driver
// starts to think that all those mongos's are one physical instance with different addresses.
// This breaks fetching large collections with "CursorNotFound" error.
// So here we manually resolve all given addresses to final IP address list and
// pass it to the mongo driver. That makes him treat each address as separate mongos server.
func prepareAddresses(srcAddresses []string) ([]string, error) {
	var ret []string
	for _, address := range srcAddresses {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, errors.Wrap(err, "net.SplitHostPort")
		}

		ips, err := net.LookupIP(host)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot resolve \"%s\"", address)
		}

		for _, ip := range ips {
			if ip.To4() != nil {
				// ipv4 case: ip:port
				ret = append(ret, ip.String()+":"+port)
			} else {
				// ipv6 case: [ip]:port
				ret = append(ret, "["+ip.String()+"]:"+port)
			}
		}
	}

	return ret, nil
}

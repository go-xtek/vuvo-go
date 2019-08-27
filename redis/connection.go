package redis

import (
	"github.com/go-xtek/vuvo-go/l"
)

// ConnectRedis ...
func ConnectRedis(uri string) Store {
	redisStore := NewWithPool(uri)
	_, err := redisStore.GetString("_test_")
	if err != nil {
		ll.Fatal("Unable to connect to Redis", l.Error(err), l.String("ConnectionString", uri))
	}
	return redisStore
}

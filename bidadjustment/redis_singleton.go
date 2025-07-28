package bidadjustment

import (
	"sync"
)

var (
	redisClientInstance *Client
	once                sync.Once
)

// GetRedisClient returns a singleton Redis client instance
func GetRedisClient() *Client {
	once.Do(func() {
		// For cluster mode
		redisClientInstance = NewClusterClient(
			[]string{"ad-server-globalb.ay4fls.clustercfg.euc1.cache.amazonaws.com:6379"},
			"", // password
		)

		//Cluster name : ad-server-global
		//redisClient := NewClient("ad-server-globalb.ay4fls.clustercfg.euc1.cache.amazonaws.com:6379", "", 0)

		//Cluster name : ad-server-uid
		//redisClient := NewClient("ad-server-uid.ay4fls.clustercfg.euc1.cache.amazonaws.com:6379", "", 0)

		//Cluster name : staging
		//redisClient := NewClient("ad-server-stage.ay4fls.clustercfg.euc1.cache.amazonaws.com:6379", "", 0)

		// For non-cluster mode
		// redisClientInstance = NewClient(
		//    "ad-server-globalb.ay4fls.clustercfg.euc1.cache.amazonaws.com:6379",
		//    "", // password
		//    0,  // db
		// )
	})
	return redisClientInstance
}

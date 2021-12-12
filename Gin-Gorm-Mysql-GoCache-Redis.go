
package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	//cache "github.com/chenyahui/gin-cache"
	"log"
	"net/http"
	"time"

	_ "github.com/lib/pq"

	"github.com/go-redis/redis"
	"github.com/gorilla/mux"
	"github.com/patrickmn/go-cache"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var myMCache CacheItf
var myRCache CacheItf


var localDB *sql.DB
var DB *gorm.DB

var err error

func main() {
	//InitMySqlDB()
	//InitPostgreDB()
	InitialMigrationForStaging()
	InitCache()      // comment if want to use redis cache
	InitRedisCache() // comment if want to use app cache


	r := mux.NewRouter()
	r.HandleFunc("/post", GetPost).Methods("GET")
	http.Handle("/", r)
	srv := &http.Server{
		Handler: r,
		Addr:    "127.0.0.1:8000",
	}

	log.Fatal(srv.ListenAndServe())
}

type CacheItf interface {
	Set(key string, data interface{}, expiration time.Duration) error
	Get(key string) ([]byte, error)
}

type AppCache struct {
	client *cache.Cache
}
type RedisCache struct {
	client *redis.Client

}
func (r *AppCache) Set(key string, data interface{}, expiration time.Duration) error {
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}

	fmt.Println("set memory cache")
	r.client.Set(key, b, expiration)
	return nil
}

func (r *AppCache) Get(key string) ([]byte, error) {
	res, exist := r.client.Get(key)
	if !exist {
		return nil, nil
	}

	fmt.Println("GET FROM IN MEMORY CACHE")
	resByte, ok := res.([]byte)
	if !ok {
		return nil, errors.New("Format is not arr of bytes")
	}

	return resByte, nil
}
func (r *RedisCache) Set(key string, data interface{}, expiration time.Duration) error {
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}

	fmt.Println("set redis")
	return r.client.Set(key, b, expiration).Err()
}

func (r *RedisCache) Get(key string) ([]byte, error) {
	result, err := r.client.Get(key).Bytes()
	if err == redis.Nil {
		return nil, nil
	}

	fmt.Println("get from  redis")

	return result, err
}



type ToDo struct {
	UserID int    `json:"userId"`
	ID     int    `json:"id"`
	Title  string `json:"title"`
	Body   string `json:"body"`
}

func GetPost(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	var result ToDo

	M, err := myMCache.Get("todo")
	if err != nil {
		// error
		log.Fatal(err)
	}
	R, err := myRCache.Get("todo")
	if err != nil {
		// error
		log.Fatal(err)
	}

	if M != nil {
		// cache exist
		err := json.Unmarshal(M, &result)
		if err != nil {
			log.Fatal(err)
		}

		M, _ := json.Marshal(map[string]interface{}{
			"data":    result,
			"elapsed": time.Since(start).Microseconds(),
		})
		w.Write([]byte(M))
		return
	}
	if R != nil {
		// cache exist
		err := json.Unmarshal(R, &result)
		if err != nil {
			log.Fatal(err)
		}

		R, _ := json.Marshal(map[string]interface{}{
			"data":    result,
			"elapsed": time.Since(start).Microseconds(),
		})
		w.Write([]byte(R))
		return
	}

	// Get from DB
//	err = localDB.QueryRow(`SELECT id, user_id, title, body FROM posts WHERE id = $1`, 1).Scan(&result.ID, &result.UserID, &result.Title, &result.Body)

	err = DB.Raw(`SELECT id,userId,title,body FROM accounts WHERE id = $1`, 2).Error
	fmt.Println("get from db")
	if err != nil {
		log.Fatal(err)
	}

	err = myMCache.Set("todo", result, 1*time.Minute)
	if err != nil {
		log.Fatal(err)
	}

	M, err = json.Marshal(map[string]interface{}{
		"data":    result,
		"elapsed": time.Since(start).Microseconds(),
	})

	if err != nil {
		log.Fatal(err)
	}

	w.Write(M)

	err = myRCache.Set("todo", result, 1*time.Minute)
	if err != nil {
		log.Fatal(err)
	}

	R, err = json.Marshal(map[string]interface{}{
		"data":    result,
		"elapsed": time.Since(start).Microseconds(),
	})

	if err != nil {
		log.Fatal(err)
	}

	w.Write(R)
}

func InitRedisCache() {
	myRCache = &RedisCache{
		client: redis.NewClient(&redis.Options{
			Addr:     "localhost:6379",
			Password: "", // no password set
			DB:       0,  // use default DB
		}),
	}

}

func InitCache() {
	myMCache = &AppCache{
		client: cache.New(5*time.Minute, 10*time.Minute),
	}
}

func InitialMigrationForStaging() {
	dsn := "root:sifat@tcp(127.0.0.1:3306)/toffee"
	DB, err = gorm.Open(mysql.Open(dsn))

	if err != nil {
		fmt.Println(err.Error())
		panic("Can't connect to DB!")
	} else {
		fmt.Println("DB connection successfull.")
	}
	//DBConfig.AutoMigrate(&entity.Video{}, entity.Author{})
}
func InitPostgreDB() {
	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s "+
		"password=%s dbname=%s sslmode=disable",
		"localhost", 5432, "postgres", "sifat", "one")

	db, err := sql.Open("postgres", psqlInfo)
	if err != nil {
		panic(err)
	}

	err = db.Ping()
	if err != nil {
		panic(err)
	}

	localDB = db
}
